package allocator

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"agones-pubsub-allocator/metrics"
	"agones-pubsub-allocator/queues"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
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
	queueManager    *QueueManager
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

// publishQueued builds and publishes a queued AllocationResult with metrics.
func (c *Controller) publishQueued(ctx context.Context, req *queues.AllocationRequest, start time.Time, queueID string, position int) error {
	status := queues.StatusQueued
	duration := time.Since(start)
	metrics.AllocationDuration.Observe(duration.Seconds())
	metrics.AllocationsTotal.WithLabelValues(string(status)).Inc()
	res := &queues.AllocationResult{
		EnvelopeVersion: "1.0",
		Type:            "allocation-result",
		TicketID:        req.TicketID,
		Status:          status,
		Token:           nil,
		ErrorMessage:    nil,
		QueuePosition:   &position,
		QueueID:         &queueID,
	}
	if err := c.publisher.PublishResult(ctx, res); err != nil {
		log.Error().Err(err).Str("ticketId", req.TicketID).Msg("controller: failed to publish queued result")
		return err
	}

	return nil
}

func (c *Controller) Handle(ctx context.Context, req *queues.AllocationRequest) error {
	start := time.Now()
	log.Info().Str("ticketId", req.TicketID).Str("fleet", req.Fleet).Msg("controller: handling allocation request")

	// Validate PlayerID is present (required for Quilkin token)
	if req.PlayerID == "" {
		log.Error().Str("ticketId", req.TicketID).Msg("controller: playerID is required for token generation")
		return c.publishFailure(ctx, req, start, "playerID is required for allocation")
	}

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

	ns := c.targetNamespace
	if ns == "" {
		ns = "default"
	}

	// Build Quilkin token for this player
	tok := buildQuilkinToken(req.PlayerID)

	// STEP 1: Check if player already has an existing allocation
	log.Info().Str("playerId", req.PlayerID).Msg("controller: checking for existing player allocation")
	existingGS, err := c.findGameServerWithToken(ctx, ns, req.Fleet, tok)
	if err != nil {
		log.Error().Err(err).Msg("controller: failed to search for existing allocation")
		return c.publishFailure(ctx, req, start, fmt.Sprintf("failed to search for existing allocation: %v", err))
	}

	// If player already has an allocated server, return the existing token
	if existingGS != nil && existingGS.Status.State == agonesv1.GameServerStateAllocated {
		log.Info().Str("gameServerName", existingGS.Name).Str("playerId", req.PlayerID).Msg("controller: found existing allocation, returning existing token")
		
		// Get address and port
		addr := existingGS.Status.Address
		var port int32
		if len(existingGS.Status.Ports) > 0 {
			port = existingGS.Status.Ports[0].Port
		}
		
		return c.publishSuccess(ctx, req, start, tok, addr, port)
	}

	// STEP 2: No valid existing allocation found, clean up any stale tokens
	log.Info().Str("playerId", req.PlayerID).Msg("controller: cleaning up existing player tokens across fleet")
	if err := c.removeTokenFromAllGameServers(ctx, ns, req.Fleet, tok); err != nil {
		log.Error().Err(err).Msg("controller: failed to cleanup player tokens, continuing with allocation")
		// Continue with allocation even if cleanup fails
	}

	// STEP 3: Check for friend joining scenario
	if len(req.JoinOnIDs) > 0 {
		log.Info().Strs("joinOnIds", req.JoinOnIDs).Bool("canJoinNotFound", req.CanJoinNotFound).Msg("controller: friend join request")

		// Build tokens for all friends
		friendTokens := make([]string, len(req.JoinOnIDs))
		for i, friendID := range req.JoinOnIDs {
			friendTokens[i] = buildQuilkinToken(friendID)
		}

		// Find gameservers with friend tokens
		gsWithFriends, err := c.findGameServersWithFriendTokens(ctx, ns, req.Fleet, friendTokens)
		if err != nil {
			log.Error().Err(err).Msg("controller: failed to search for friend gameservers")
			return c.publishFailure(ctx, req, start, fmt.Sprintf("failed to search for friends: %v", err))
		}

		if len(gsWithFriends) > 0 {
			// Friends found on one or more gameservers
			// Pick the first one (could be enhanced with better selection logic)
			var targetGS string
			for gsName := range gsWithFriends {
				targetGS = gsName
				break
			}

			log.Info().Str("gameServerName", targetGS).Strs("friendsFound", gsWithFriends[targetGS]).Msg("controller: found friends on gameserver")

			// Try to join the friend's gameserver
			return c.joinExistingGameServer(ctx, req, start, ns, targetGS, tok)
		}

		// Friends not found
		if !req.CanJoinNotFound {
			// Player cannot join without friends, fail the request
			log.Info().Str("ticketId", req.TicketID).Msg("controller: friends not found and canJoinNotFound=false")
			return c.publishFailure(ctx, req, start, "friends not found on any gameserver")
		}

		// Friends not found but canJoinNotFound=true, proceed with normal allocation
		log.Info().Str("ticketId", req.TicketID).Msg("controller: friends not found but canJoinNotFound=true, proceeding with normal allocation")
	}

	// STEP 4: Normal allocation flow (no friends or canJoinNotFound=true)
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

	// Get address and port for logging/validation
	addr := created.Status.Address
	var port int32
	if len(created.Status.Ports) > 0 {
		port = created.Status.Ports[0].Port
	}
	if addr == "" || port == 0 {
		log.Error().Str("address", addr).Int32("port", port).Msg("controller: allocated GameServer missing address/port")
		return c.publishFailure(ctx, req, start, "allocated GameServer missing address/port")
	}

	// Add token to GameServer annotations for quilkin
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

	// Add the token to its annotations (append if exists, create if not)
	if gs.ObjectMeta.Annotations == nil {
		gs.ObjectMeta.Annotations = make(map[string]string)
	}
	gs.ObjectMeta.Annotations["quilkin.dev/tokens"] = appendToken(gs.ObjectMeta.Annotations["quilkin.dev/tokens"], tok)
	log.Info().Str("gameServerName", gameServerName).Str("playerId", req.PlayerID).Str("token", tok).Msg("controller: updating GameServer with routing token")

	// Update the GameServer object in the cluster
	_, err = c.agones.AgonesV1().GameServers(ns).Update(ctx, gs, metav1.UpdateOptions{})
	if err != nil {
		log.Error().Err(err).Str("namespace", ns).Str("gameServerName", gameServerName).Msg("controller: failed to update GameServer with token")
		return c.publishFailure(ctx, req, start, fmt.Sprintf("failed to update GameServer with token: %v", err))
	}

	return c.publishSuccess(ctx, req, start, tok, addr, port)
}

// joinExistingGameServer attempts to add a player to an existing gameserver
// If the server is full, the player is queued
func (c *Controller) joinExistingGameServer(ctx context.Context, req *queues.AllocationRequest, start time.Time, namespace, gameServerName, token string) error {
	// Get the gameserver
	gs, err := c.agones.AgonesV1().GameServers(namespace).Get(ctx, gameServerName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).Str("gameServerName", gameServerName).Msg("controller: failed to get friend's gameserver")
		return c.publishFailure(ctx, req, start, fmt.Sprintf("failed to get friend's gameserver: %v", err))
	}

	// Check if gameserver is allocated
	if gs.Status.State != agonesv1.GameServerStateAllocated {
		log.Warn().Str("gameServerName", gameServerName).Str("state", string(gs.Status.State)).Msg("controller: friend's gameserver not in allocated state")
		return c.publishFailure(ctx, req, start, "friend's gameserver is not available")
	}

	// TODO: Check if server has capacity (this would require game-specific logic)
	// For now, we'll assume we can add the token and let the game server handle capacity
	// In a production system, you'd check player count vs max players here

	// Add player's token to the gameserver
	if gs.ObjectMeta.Annotations == nil {
		gs.ObjectMeta.Annotations = make(map[string]string)
	}
	gs.ObjectMeta.Annotations["quilkin.dev/tokens"] = appendToken(gs.ObjectMeta.Annotations["quilkin.dev/tokens"], token)
	log.Info().Str("gameServerName", gameServerName).Str("playerId", req.PlayerID).Str("token", token).Msg("controller: adding player to friend's gameserver")

	_, err = c.agones.AgonesV1().GameServers(namespace).Update(ctx, gs, metav1.UpdateOptions{})
	if err != nil {
		log.Error().Err(err).Str("gameServerName", gameServerName).Msg("controller: failed to add token to friend's gameserver")
		return c.publishFailure(ctx, req, start, fmt.Sprintf("failed to join friend's gameserver: %v", err))
	}

	// Get address and port
	addr := gs.Status.Address
	var port int32
	if len(gs.Status.Ports) > 0 {
		port = gs.Status.Ports[0].Port
	}

	return c.publishSuccess(ctx, req, start, token, addr, port)
}

func NewController(p queues.Publisher, ns string) *Controller {
	return &Controller{
		publisher:       p,
		targetNamespace: ns,
		queueManager:    NewQueueManager(),
	}
}

// appendToken adds a new token to a comma-separated list of tokens.
// If the token already exists in the list, it won't be added again.
// Returns the updated comma-separated token list.
func appendToken(existingTokens, newToken string) string {
	if existingTokens == "" {
		return newToken
	}

	// Split existing tokens and check for duplicates
	tokens := splitAndTrim(existingTokens)
	for _, t := range tokens {
		if t == newToken {
			// Token already exists, return as-is
			return existingTokens
		}
	}

	// Append new token
	return existingTokens + "," + newToken
}

// splitAndTrim splits a comma-separated string and trims whitespace from each element.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// findGameServerWithToken searches for a GameServer in the fleet that has the specified token.
// Returns nil if no GameServer is found with the token.
func (c *Controller) findGameServerWithToken(ctx context.Context, namespace, fleet, token string) (*agonesv1.GameServer, error) {
	// List all GameServers in the fleet
	gsList, err := c.agones.AgonesV1().GameServers(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("agones.dev/fleet=%s", fleet),
	})
	if err != nil {
		return nil, err
	}

	// Search for a GameServer with this token in its annotations
	for i := range gsList.Items {
		gs := &gsList.Items[i]
		if gs.ObjectMeta.Annotations == nil {
			continue
		}
		tokens := gs.ObjectMeta.Annotations["quilkin.dev/tokens"]
		if tokens == "" {
			continue
		}
		// Check if our token is in the comma-separated list
		tokenList := splitAndTrim(tokens)
		for _, t := range tokenList {
			if t == token {
				return gs, nil
			}
		}
	}

	return nil, nil
}

// removeTokenFromAllGameServers removes a player's token from all gameservers in the fleet
// This ensures a player only has one active server allocation at a time
func (c *Controller) removeTokenFromAllGameServers(ctx context.Context, namespace, fleet, token string) error {
	gsList, err := c.agones.AgonesV1().GameServers(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("agones.dev/fleet=%s", fleet),
	})
	if err != nil {
		return err
	}

	for i := range gsList.Items {
		gs := &gsList.Items[i]
		if gs.ObjectMeta.Annotations == nil {
			continue
		}

		tokens := gs.ObjectMeta.Annotations["quilkin.dev/tokens"]
		if tokens == "" {
			continue
		}

		// Check if this gameserver has the token
		tokenList := splitAndTrim(tokens)
		hasToken := false
		for _, t := range tokenList {
			if t == token {
				hasToken = true
				break
			}
		}

		if !hasToken {
			continue
		}

		// Remove the token from the list
		newTokens := removeToken(tokens, token)
		gs.ObjectMeta.Annotations["quilkin.dev/tokens"] = newTokens

		log.Info().Str("gameServerName", gs.Name).Str("token", token).Msg("controller: removing token from GameServer")

		_, err := c.agones.AgonesV1().GameServers(namespace).Update(ctx, gs, metav1.UpdateOptions{})
		if err != nil {
			log.Error().Err(err).Str("gameServerName", gs.Name).Msg("controller: failed to remove token from GameServer")
			// Continue with other servers even if one fails
		}
	}

	return nil
}

// removeToken removes a specific token from a comma-separated list of tokens
func removeToken(existingTokens, tokenToRemove string) string {
	if existingTokens == "" {
		return ""
	}

	tokenList := splitAndTrim(existingTokens)
	var newTokens []string

	for _, t := range tokenList {
		if t != tokenToRemove {
			newTokens = append(newTokens, t)
		}
	}

	return strings.Join(newTokens, ",")
}

// findGameServersWithFriendTokens searches for gameservers that have any of the friend tokens
// Returns a map of gameserver to the list of friend tokens found on it
func (c *Controller) findGameServersWithFriendTokens(ctx context.Context, namespace, fleet string, friendTokens []string) (map[string][]string, error) {
	gsList, err := c.agones.AgonesV1().GameServers(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("agones.dev/fleet=%s", fleet),
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)

	for i := range gsList.Items {
		gs := &gsList.Items[i]
		if gs.ObjectMeta.Annotations == nil {
			continue
		}

		tokens := gs.ObjectMeta.Annotations["quilkin.dev/tokens"]
		if tokens == "" {
			continue
		}

		tokenList := splitAndTrim(tokens)
		var foundFriends []string

		for _, friendToken := range friendTokens {
			for _, t := range tokenList {
				if t == friendToken {
					foundFriends = append(foundFriends, friendToken)
					break
				}
			}
		}

		if len(foundFriends) > 0 {
			result[gs.Name] = foundFriends
		}
	}

	return result, nil
}

// publishSuccess builds and publishes a success AllocationResult with metrics.
func (c *Controller) publishSuccess(ctx context.Context, req *queues.AllocationRequest, start time.Time, token, addr string, port int32) error {
	status := queues.StatusSuccess
	duration := time.Since(start)
	metrics.AllocationDuration.Observe(duration.Seconds())
	metrics.AllocationsTotal.WithLabelValues(string(status)).Inc()

	res := &queues.AllocationResult{
		EnvelopeVersion: "1.0",
		Type:            "allocation-result",
		TicketID:        req.TicketID,
		Status:          status,
		Token:           &token,
		ErrorMessage:    nil,
	}
	if err := c.publisher.PublishResult(ctx, res); err != nil {
		log.Error().Err(err).Str("ticketId", req.TicketID).Dur("duration", duration).Msg("controller: failed to publish result")
		return err
	}
	log.Info().Str("ticketId", req.TicketID).Str("status", string(status)).Dur("duration", duration).Str("addr", addr).Int32("port", port).Msg("controller: allocation successful")
	return nil
}

// buildQuilkinToken creates a 16-byte token from playerID.
// The playerID is truncated or padded to fit exactly 16 bytes, then base64 encoded.
func buildQuilkinToken(playerID string) string {
	const tokenSize = 16
	// Create a 16-byte buffer
	buf := make([]byte, tokenSize)

	// Copy playerID into buffer (truncate if too long, pad with zeros if too short)
	playerBytes := []byte(playerID)
	if len(playerBytes) > tokenSize {
		playerBytes = playerBytes[:tokenSize]
	}
	copy(buf, playerBytes)

	// Base64 encode the 16-byte buffer
	return base64.StdEncoding.EncodeToString(buf)
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
