package actor

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

const (
	passivationTimeout = 5 * time.Minute
	snapshotInterval   = 10
)

// System manages the Proto.Actor ActorSystem and spawned aggregate actors.
type System struct {
	actorSystem *actor.ActorSystem
	rootContext *actor.RootContext
	provider    *actorpersistence.DynamoProvider
	actors      sync.Map // "AggregateType:ID" → *actor.PID
	readStore   store.ReadStoreInterface
}

// NewSystem creates and initializes the actor system.
func NewSystem(eventStore store.EventStoreInterface, readStore store.ReadStoreInterface) *System {
	system := actor.NewActorSystem()
	provider := actorpersistence.NewDynamoProvider(eventStore, snapshotInterval)

	return &System{
		actorSystem: system,
		rootContext: system.Root,
		provider:    provider,
		readStore:   readStore,
	}
}

// Root returns the RootContext for sending messages to actors.
func (s *System) Root() *actor.RootContext {
	return s.rootContext
}

// ActorSystem returns the underlying Proto.Actor system.
func (s *System) ActorSystem() *actor.ActorSystem {
	return s.actorSystem
}

// ProducerFunc is a function that creates a new actor instance.
type ProducerFunc func() actor.Actor

// GetOrSpawn retrieves an existing actor or spawns a new one.
// actorName should be in the format "AggregateType:AggregateID".
func (s *System) GetOrSpawn(actorName string, producer ProducerFunc) *actor.PID {
	if pid, ok := s.actors.Load(actorName); ok {
		return pid.(*actor.PID)
	}

	props := actor.PropsFromProducer(
		func() actor.Actor { return producer() },
		actor.WithReceiverMiddleware(persistence.Using(s.provider)),
	)

	pid, err := s.rootContext.SpawnNamed(props, actorName)
	if err != nil {
		slog.Error("failed to spawn actor", "name", actorName, "error", err)
		// Actor may already exist if we hit a race
		if existing, ok := s.actors.Load(actorName); ok {
			return existing.(*actor.PID)
		}
		return nil
	}

	s.actors.Store(actorName, pid)
	return pid
}

// RemoveActor removes a passivated actor from the registry.
func (s *System) RemoveActor(actorName string) {
	s.actors.Delete(actorName)
}

// GetOrSpawnProduct spawns or retrieves a Product actor.
func (s *System) GetOrSpawnProduct(productID string, producer ProducerFunc) *actor.PID {
	name := actorpersistence.FormatActorName("Product", productID)
	return s.GetOrSpawn(name, producer)
}

// GetOrSpawnInventory spawns or retrieves an Inventory actor.
func (s *System) GetOrSpawnInventory(productID string, producer ProducerFunc) *actor.PID {
	name := actorpersistence.FormatActorName("Inventory", productID)
	return s.GetOrSpawn(name, producer)
}

// GetOrSpawnCart spawns or retrieves a Cart actor.
func (s *System) GetOrSpawnCart(userID string, producer ProducerFunc) *actor.PID {
	cartID := fmt.Sprintf("cart-%s", userID)
	name := actorpersistence.FormatActorName("Cart", cartID)
	return s.GetOrSpawn(name, producer)
}

// GetOrSpawnOrder spawns or retrieves an Order actor.
func (s *System) GetOrSpawnOrder(orderID string, producer ProducerFunc) *actor.PID {
	name := actorpersistence.FormatActorName("Order", orderID)
	return s.GetOrSpawn(name, producer)
}

// GetOrSpawnUser spawns or retrieves a User actor.
func (s *System) GetOrSpawnUser(userID string, producer ProducerFunc) *actor.PID {
	name := actorpersistence.FormatActorName("User", userID)
	return s.GetOrSpawn(name, producer)
}

// GetOrSpawnCategory spawns or retrieves a Category actor.
func (s *System) GetOrSpawnCategory(categoryID string, producer ProducerFunc) *actor.PID {
	name := actorpersistence.FormatActorName("Category", categoryID)
	return s.GetOrSpawn(name, producer)
}

// Shutdown gracefully shuts down the actor system.
func (s *System) Shutdown() {
	slog.Info("shutting down actor system")
	s.actorSystem.Shutdown()
}
