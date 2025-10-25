package allocator

import (
	"sync"
	"time"

	"agones-pubsub-allocator/queues"
)

// QueueEntry represents a player waiting in queue for a gameserver
type QueueEntry struct {
	Request   *queues.AllocationRequest
	Timestamp time.Time
	Position  int
}

// QueueManager manages FIFO queues for gameserver allocation
// Since this allocator is intended to be a single pod, we store queues in memory
type QueueManager struct {
	mu     sync.RWMutex
	queues map[string][]*QueueEntry // key: gameserver name, value: queue of players
}

// NewQueueManager creates a new queue manager
func NewQueueManager() *QueueManager {
	return &QueueManager{
		queues: make(map[string][]*QueueEntry),
	}
}

// Enqueue adds a player to the queue for a specific gameserver
func (qm *QueueManager) Enqueue(gameServerName string, req *queues.AllocationRequest) int {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	entry := &QueueEntry{
		Request:   req,
		Timestamp: time.Now(),
	}

	qm.queues[gameServerName] = append(qm.queues[gameServerName], entry)

	// Update positions for all entries in this queue
	for i, e := range qm.queues[gameServerName] {
		e.Position = i + 1
	}

	return entry.Position
}

// Dequeue removes and returns the first player in the queue for a gameserver
func (qm *QueueManager) Dequeue(gameServerName string) *QueueEntry {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	queue, exists := qm.queues[gameServerName]
	if !exists || len(queue) == 0 {
		return nil
	}

	entry := queue[0]
	qm.queues[gameServerName] = queue[1:]

	// Update positions for remaining entries
	for i, e := range qm.queues[gameServerName] {
		e.Position = i + 1
	}

	return entry
}

// GetPosition returns the current position of a player in the queue
func (qm *QueueManager) GetPosition(gameServerName, ticketID string) (int, bool) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	queue, exists := qm.queues[gameServerName]
	if !exists {
		return 0, false
	}

	for _, entry := range queue {
		if entry.Request.TicketID == ticketID {
			return entry.Position, true
		}
	}

	return 0, false
}

// RemoveFromQueue removes a specific player from the queue (e.g., if they disconnect)
func (qm *QueueManager) RemoveFromQueue(gameServerName, ticketID string) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	queue, exists := qm.queues[gameServerName]
	if !exists {
		return false
	}

	for i, entry := range queue {
		if entry.Request.TicketID == ticketID {
			qm.queues[gameServerName] = append(queue[:i], queue[i+1:]...)

			// Update positions for remaining entries
			for j, e := range qm.queues[gameServerName] {
				e.Position = j + 1
			}

			return true
		}
	}

	return false
}

// GetQueueLength returns the number of players waiting for a gameserver
func (qm *QueueManager) GetQueueLength(gameServerName string) int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	queue, exists := qm.queues[gameServerName]
	if !exists {
		return 0
	}

	return len(queue)
}

// ClearQueue removes all entries from a gameserver's queue
func (qm *QueueManager) ClearQueue(gameServerName string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	delete(qm.queues, gameServerName)
}

// GetAllQueues returns a snapshot of all queues (for monitoring/debugging)
func (qm *QueueManager) GetAllQueues() map[string]int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	snapshot := make(map[string]int)
	for gsName, queue := range qm.queues {
		snapshot[gsName] = len(queue)
	}

	return snapshot
}
