package allocator

import (
	"testing"

	"agones-pubsub-allocator/queues"
)

func TestNewQueueManager(t *testing.T) {
	qm := NewQueueManager()
	if qm == nil {
		t.Fatal("NewQueueManager() returned nil")
	}
	if qm.queues == nil {
		t.Error("queues map not initialized")
	}
}

func TestQueueManager_Enqueue(t *testing.T) {
	qm := NewQueueManager()
	gsName := "test-gameserver"

	req1 := &queues.AllocationRequest{TicketID: "ticket1", PlayerID: "player1"}
	req2 := &queues.AllocationRequest{TicketID: "ticket2", PlayerID: "player2"}
	req3 := &queues.AllocationRequest{TicketID: "ticket3", PlayerID: "player3"}

	pos1 := qm.Enqueue(gsName, req1)
	if pos1 != 1 {
		t.Errorf("First enqueue position = %d, want 1", pos1)
	}

	pos2 := qm.Enqueue(gsName, req2)
	if pos2 != 2 {
		t.Errorf("Second enqueue position = %d, want 2", pos2)
	}

	pos3 := qm.Enqueue(gsName, req3)
	if pos3 != 3 {
		t.Errorf("Third enqueue position = %d, want 3", pos3)
	}

	length := qm.GetQueueLength(gsName)
	if length != 3 {
		t.Errorf("Queue length = %d, want 3", length)
	}
}

func TestQueueManager_Dequeue(t *testing.T) {
	qm := NewQueueManager()
	gsName := "test-gameserver"

	req1 := &queues.AllocationRequest{TicketID: "ticket1", PlayerID: "player1"}
	req2 := &queues.AllocationRequest{TicketID: "ticket2", PlayerID: "player2"}

	qm.Enqueue(gsName, req1)
	qm.Enqueue(gsName, req2)

	entry := qm.Dequeue(gsName)
	if entry == nil {
		t.Fatal("Dequeue() returned nil")
	}
	if entry.Request.TicketID != "ticket1" {
		t.Errorf("Dequeued ticket = %s, want ticket1", entry.Request.TicketID)
	}

	length := qm.GetQueueLength(gsName)
	if length != 1 {
		t.Errorf("Queue length after dequeue = %d, want 1", length)
	}

	// Verify position updated for remaining entry
	pos, found := qm.GetPosition(gsName, "ticket2")
	if !found {
		t.Error("ticket2 not found in queue")
	}
	if pos != 1 {
		t.Errorf("ticket2 position = %d, want 1", pos)
	}
}

func TestQueueManager_DequeueEmpty(t *testing.T) {
	qm := NewQueueManager()
	gsName := "test-gameserver"

	entry := qm.Dequeue(gsName)
	if entry != nil {
		t.Errorf("Dequeue() on empty queue returned %v, want nil", entry)
	}
}

func TestQueueManager_GetPosition(t *testing.T) {
	qm := NewQueueManager()
	gsName := "test-gameserver"

	req1 := &queues.AllocationRequest{TicketID: "ticket1", PlayerID: "player1"}
	req2 := &queues.AllocationRequest{TicketID: "ticket2", PlayerID: "player2"}

	qm.Enqueue(gsName, req1)
	qm.Enqueue(gsName, req2)

	pos, found := qm.GetPosition(gsName, "ticket1")
	if !found {
		t.Error("ticket1 not found")
	}
	if pos != 1 {
		t.Errorf("ticket1 position = %d, want 1", pos)
	}

	pos, found = qm.GetPosition(gsName, "ticket2")
	if !found {
		t.Error("ticket2 not found")
	}
	if pos != 2 {
		t.Errorf("ticket2 position = %d, want 2", pos)
	}

	_, found = qm.GetPosition(gsName, "nonexistent")
	if found {
		t.Error("nonexistent ticket found")
	}
}

func TestQueueManager_RemoveFromQueue(t *testing.T) {
	qm := NewQueueManager()
	gsName := "test-gameserver"

	req1 := &queues.AllocationRequest{TicketID: "ticket1", PlayerID: "player1"}
	req2 := &queues.AllocationRequest{TicketID: "ticket2", PlayerID: "player2"}
	req3 := &queues.AllocationRequest{TicketID: "ticket3", PlayerID: "player3"}

	qm.Enqueue(gsName, req1)
	qm.Enqueue(gsName, req2)
	qm.Enqueue(gsName, req3)

	// Remove middle entry
	removed := qm.RemoveFromQueue(gsName, "ticket2")
	if !removed {
		t.Error("RemoveFromQueue() returned false, want true")
	}

	length := qm.GetQueueLength(gsName)
	if length != 2 {
		t.Errorf("Queue length after remove = %d, want 2", length)
	}

	// Verify positions updated
	pos, found := qm.GetPosition(gsName, "ticket3")
	if !found {
		t.Error("ticket3 not found")
	}
	if pos != 2 {
		t.Errorf("ticket3 position after remove = %d, want 2", pos)
	}

	// Try to remove nonexistent
	removed = qm.RemoveFromQueue(gsName, "nonexistent")
	if removed {
		t.Error("RemoveFromQueue() for nonexistent returned true, want false")
	}
}

func TestQueueManager_ClearQueue(t *testing.T) {
	qm := NewQueueManager()
	gsName := "test-gameserver"

	req1 := &queues.AllocationRequest{TicketID: "ticket1", PlayerID: "player1"}
	req2 := &queues.AllocationRequest{TicketID: "ticket2", PlayerID: "player2"}

	qm.Enqueue(gsName, req1)
	qm.Enqueue(gsName, req2)

	qm.ClearQueue(gsName)

	length := qm.GetQueueLength(gsName)
	if length != 0 {
		t.Errorf("Queue length after clear = %d, want 0", length)
	}
}

func TestQueueManager_GetAllQueues(t *testing.T) {
	qm := NewQueueManager()

	req1 := &queues.AllocationRequest{TicketID: "ticket1", PlayerID: "player1"}
	req2 := &queues.AllocationRequest{TicketID: "ticket2", PlayerID: "player2"}

	qm.Enqueue("gs1", req1)
	qm.Enqueue("gs1", req2)
	qm.Enqueue("gs2", req1)

	snapshot := qm.GetAllQueues()
	if len(snapshot) != 2 {
		t.Errorf("GetAllQueues() returned %d queues, want 2", len(snapshot))
	}
	if snapshot["gs1"] != 2 {
		t.Errorf("gs1 queue length = %d, want 2", snapshot["gs1"])
	}
	if snapshot["gs2"] != 1 {
		t.Errorf("gs2 queue length = %d, want 1", snapshot["gs2"])
	}
}

func TestQueueManager_Concurrent(t *testing.T) {
	qm := NewQueueManager()
	gsName := "test-gameserver"

	// Test concurrent enqueues
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			req := &queues.AllocationRequest{
				TicketID: string(rune('A' + id)),
				PlayerID: string(rune('A' + id)),
			}
			qm.Enqueue(gsName, req)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	length := qm.GetQueueLength(gsName)
	if length != 10 {
		t.Errorf("Queue length after concurrent enqueues = %d, want 10", length)
	}
}
