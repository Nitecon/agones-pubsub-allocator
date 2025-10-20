package allocator

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"agones-pubsub-allocator/metrics"
	"agones-pubsub-allocator/queues"

	allocationv1 "agones.dev/agones/pkg/apis/allocation/v1"
	agonesclientset "agones.dev/agones/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Controller wires queue consumption to the allocation execution
// Allocation execution (Agones interaction) will be implemented in a follow-up
type Controller struct {
	publisher       queues.Publisher
	targetNamespace string
	agones          agonesclientset.Interface
}

// publishFailure builds and publishes a failure AllocationResult with metrics.
func (c *Controller) publishFailure(ctx context.Context, req *queues.AllocationRequest, start time.Time, message string) error {
	status := queues.StatusFailure
	duration := time.Since(start)
	metrics.AllocationDuration.Observe(duration.Seconds())
	metrics.AllocationsTotal.WithLabelValues(string(status)).Inc()
	res := &queues.AllocationResult{
		EnvelopeVersion: "1.0",
		Type:            "allocation-result",
		TicketID:        req.TicketID,
		Status:          status,
		Token:           nil,
		ErrorMessage:    &message,
	}
	if err := c.publisher.PublishResult(ctx, res); err != nil {
		log.Error().Err(err).Str("ticketId", req.TicketID).Msg("controller: failed to publish failure result")
		return err
	}

	return nil
}

func (c *Controller) Handle(ctx context.Context, req *queues.AllocationRequest) error {
	start := time.Now()
	log.Info().Str("ticketId", req.TicketID).Str("fleet", req.Fleet).Msg("controller: handling allocation request")

	// Lazy init Agones client
	if c.agones == nil {
		cli, err := newAgonesClient()
		if err != nil {
			log.Error().Err(err).Msg("controller: failed to initialize Agones client")
			return c.publishFailure(ctx, req, start, fmt.Sprintf("agones client init failed: %v", err))
		}
		c.agones = cli
		log.Info().Msg("controller: Agones client initialized")
	}

	// Build GameServerAllocation spec using fleet label from request
	gsa := &allocationv1.GameServerAllocation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: allocationv1.SchemeGroupVersion.String(),
			Kind:       "GameServerAllocation",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: allocationv1.GameServerAllocationSpec{
			Selectors: []allocationv1.GameServerSelector{
				{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"agones.dev/fleet": req.Fleet,
						},
					},
				},
			},
		},
	}

	ns := c.targetNamespace
	if ns == "" {
		ns = "default"
	}

	created, err := c.agones.AllocationV1().GameServerAllocations(ns).Create(ctx, gsa, metav1.CreateOptions{})
	if err != nil {
		log.Error().Err(err).Str("namespace", ns).Str("fleet", req.Fleet).Msg("controller: GameServerAllocation create failed")
		return c.publishFailure(ctx, req, start, fmt.Sprintf("allocation create failed: %v", err))
	}

	if created.Status.State != allocationv1.GameServerAllocationAllocated {
		msg := fmt.Sprintf("allocation not allocated (state=%s)", created.Status.State)
		log.Warn().Str("state", string(created.Status.State)).Str("namespace", ns).Msg("controller: allocation not allocated")
		return c.publishFailure(ctx, req, start, msg)
	}

	// Success: build token from Address and first Port
	addr := created.Status.Address
	var port int32
	if len(created.Status.Ports) > 0 {
		port = created.Status.Ports[0].Port
	}
	if addr == "" || port == 0 {
		log.Error().Str("address", addr).Int32("port", port).Msg("controller: allocated GameServer missing address/port")
		return c.publishFailure(ctx, req, start, "allocated GameServer missing address/port")
	}

	raw := fmt.Sprintf("%s:%d", addr, port)
	tok := base64.StdEncoding.EncodeToString([]byte(raw))

	// START: Add token to GameServer annotations for quilkin
	gameServerName := created.Status.GameServerName
	if gameServerName == "" {
		msg := "allocated GameServer name is empty in allocation response"
		log.Error().Str("namespace", ns).Msg("controller: " + msg)
		return c.publishFailure(ctx, req, start, msg)
	}

	// Get the allocated GameServer object
	gs, err := c.agones.AgonesV1().GameServers(ns).Get(ctx, gameServerName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).Str("namespace", ns).Str("gameServerName", gameServerName).Msg("controller: failed to get allocated GameServer")
		return c.publishFailure(ctx, req, start, fmt.Sprintf("failed to get GameServer '%s': %v", gameServerName, err))
	}

	// Add the token to its annotations
	if gs.ObjectMeta.Annotations == nil {
		gs.ObjectMeta.Annotations = make(map[string]string)
	}
	gs.ObjectMeta.Annotations["quilkin.dev/tokens"] = tok
	log.Info().Str("gameServerName", gameServerName).Str("token", tok).Msg("controller: updating GameServer with routing token")

	// Update the GameServer object in the cluster
	_, err = c.agones.AgonesV1().GameServers(ns).Update(ctx, gs, metav1.UpdateOptions{})
	if err != nil {
		log.Error().Err(err).Str("namespace", ns).Str("gameServerName", gameServerName).Msg("controller: failed to update GameServer with token")
		return c.publishFailure(ctx, req, start, fmt.Sprintf("failed to update GameServer with token: %v", err))
	}
	// END: Add token to GameServer annotations for quilkin

	status := queues.StatusSuccess
	duration := time.Since(start)
	metrics.AllocationDuration.Observe(duration.Seconds())
	metrics.AllocationsTotal.WithLabelValues(string(status)).Inc()

	res := &queues.AllocationResult{
		EnvelopeVersion: "1.0",
		Type:            "allocation-result",
		TicketID:        req.TicketID,
		Status:          status,
		Token:           &tok,
		ErrorMessage:    nil,
	}
	if err := c.publisher.PublishResult(ctx, res); err != nil {
		log.Error().Err(err).Str("ticketId", req.TicketID).Dur("duration", duration).Msg("controller: failed to publish result")
		return err
	}
	log.Info().Str("ticketId", req.TicketID).Str("status", string(status)).Dur("duration", duration).Str("addr", addr).Int32("port", port).Msg("controller: allocation successful")
	return nil
}

func NewController(p queues.Publisher, ns string) *Controller {
	return &Controller{publisher: p, targetNamespace: ns}
}

// newAgonesClient returns an Agones typed clientset using in-cluster config or local kubeconfig.
func newAgonesClient() (agonesclientset.Interface, error) {
	// Try in-cluster config first
	if cfg, err := rest.InClusterConfig(); err == nil {
		return agonesclientset.NewForConfig(cfg)
	}
	// Fallback to local kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return agonesclientset.NewForConfig(cfg)
}
