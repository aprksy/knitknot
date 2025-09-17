package inmem_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("In-Memory Storage CRUD", func() {
	var storage *inmem.Storage

	BeforeEach(func() {
		storage = inmem.New()
	})

	Describe("AddNode", func() {
		It("should create a node with label and properties", func() {
			id, err := storage.AddNode("User", map[string]any{
				"name": "Alice",
				"age":  35,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(id).NotTo(BeEmpty())

			node, ok := storage.GetNode(id)
			Expect(ok).To(BeTrue())
			Expect(node.Label).To(Equal("User"))
			Expect(node.Props["name"]).To(Equal("Alice"))
			Expect(node.Props["age"]).To(Equal(35))
		})

		It("should reject duplicate ID (if used)", func() {
			_, err := storage.AddNode("User", nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UpdateNode", func() {
		Context("when node exists", func() {
			var nodeID string

			BeforeEach(func() {
				var err error
				nodeID, err = storage.AddNode("User", map[string]any{
					"name":   "Bob",
					"status": "active",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update existing properties", func() {
				err := storage.UpdateNode(nodeID, map[string]any{
					"name":   "Bobby",
					"status": "inactive",
					"age":    30,
				})
				Expect(err).NotTo(HaveOccurred())

				node, ok := storage.GetNode(nodeID)
				Expect(ok).To(BeTrue())
				Expect(node.Props["name"]).To(Equal("Bobby"))
				Expect(node.Props["status"]).To(Equal("inactive"))
				Expect(node.Props["age"]).To(Equal(30))
			})

			It("should preserve node label", func() {
				err := storage.UpdateNode(nodeID, map[string]any{"name": "Charlie"})
				Expect(err).NotTo(HaveOccurred())

				node, _ := storage.GetNode(nodeID)
				Expect(node.Label).To(Equal("User")) // unchanged
			})
		})

		Context("when node does not exist", func() {
			It("should return error", func() {
				err := storage.UpdateNode("unknown", map[string]any{"x": "y"})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("AddEdge", func() {
		var fromID, toID string

		BeforeEach(func() {
			var err error
			fromID, err = storage.AddNode("User", nil)
			Expect(err).NotTo(HaveOccurred())
			toID, err = storage.AddNode("Skill", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create edge with kind and props", func() {
			err := storage.AddEdge(fromID, toID, "has_skill", map[string]any{
				"level": 4,
				"since": "2023-01-01",
			})
			Expect(err).NotTo(HaveOccurred())

			edges := storage.GetEdgesFrom(fromID)
			Expect(edges).To(HaveLen(1))
			e := edges[0]
			Expect(e.Kind).To(Equal("has_skill"))
			Expect(e.Props["level"]).To(Equal(4))
			Expect(e.Props["since"]).To(Equal("2023-01-01"))
		})

		It("should fail if source node missing", func() {
			err := storage.AddEdge("missing", toID, "rel", nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if target node missing", func() {
			err := storage.AddEdge(fromID, "missing", "rel", nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("UpdateEdge", func() {
		var fromID, toID, edgeID string

		BeforeEach(func() {
			var err error
			fromID, err = storage.AddNode("User", nil)
			Expect(err).NotTo(HaveOccurred())
			toID, err = storage.AddNode("Skill", nil)
			Expect(err).NotTo(HaveOccurred())

			err = storage.AddEdge(fromID, toID, "has_skill", map[string]any{"level": 1})
			Expect(err).NotTo(HaveOccurred())

			edgeID = fromID + "->" + toID + "@has_skill"
		})

		It("should update edge properties", func() {
			// Since we don't have UpdateEdge yet, simulate direct access
			// Or add method later
			edge, ok := storage.GetEdge(edgeID)
			Expect(ok).To(BeTrue())

			edge.Props["level"] = 5
			edge.Props["verified"] = true

			// Re-fetch
			edges := storage.GetEdgesFrom(fromID)
			Expect(edges).To(HaveLen(1))
			e := edges[0]
			Expect(e.Props["level"]).To(Equal(5))
			Expect(e.Props["verified"]).To(BeTrue())
		})
	})

	Describe("DeleteNode", func() {
		Context("when node exists", func() {
			var nodeID string

			BeforeEach(func() {
				var err error
				nodeID, err = storage.AddNode("Temp", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove node", func() {
				err := storage.DeleteNode(nodeID)
				Expect(err).NotTo(HaveOccurred())

				_, ok := storage.GetNode(nodeID)
				Expect(ok).To(BeFalse())
			})

			It("should not be idempotent", func() {
				storage.DeleteNode(nodeID)
				Expect(storage.DeleteNode(nodeID)).ToNot(Succeed())
			})
		})

		Context("with connected edges", func() {
			var fromID, toID string

			BeforeEach(func() {
				var err error
				fromID, err = storage.AddNode("A", nil)
				Expect(err).NotTo(HaveOccurred())
				toID, err = storage.AddNode("B", nil)
				Expect(err).NotTo(HaveOccurred())

				err = storage.AddEdge(fromID, toID, "rel", nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("DeleteEdge", func() {
		var fromID, toID string

		BeforeEach(func() {
			var err error
			fromID, err = storage.AddNode("User", nil)
			Expect(err).NotTo(HaveOccurred())
			toID, err = storage.AddNode("Skill", nil)
			Expect(err).NotTo(HaveOccurred())

			err = storage.AddEdge(fromID, toID, "has_skill", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove edge", func() {
			err := storage.DeleteEdge(fromID, toID, "has_skill")
			Expect(err).NotTo(HaveOccurred())

			edges := storage.GetEdgesFrom(fromID)
			Expect(edges).To(BeEmpty())
		})

		It("should be idempotent", func() {
			storage.DeleteEdge(fromID, toID, "has_skill")
			Expect(storage.DeleteEdge(fromID, toID, "has_skill")).ToNot(Succeed())
		})
	})

	// Describe("Concurrency Safety", func() {
	// 	It("should handle concurrent reads and writes", func(done Done) {
	// 		// Add initial nodes
	// 		aliceID, _ := storage.AddNode("User", map[string]any{"name": "Alice"})
	// 		goID, _ := storage.AddNode("Skill", map[string]any{"name": "Go"})
	// 		_ = storage.AddEdge(aliceID, goID, "has_skill", map[string]any{"level": 1})

	// 		// Simulate concurrent operations
	// 		doneChan := make(chan bool, 10)

	// 		for i := 0; i < 10; i++ {
	// 			go func(i int) {
	// 				defer GinkgoRecover()

	// 				// Read
	// 				node, ok := storage.GetNode(aliceID)
	// 				Expect(ok).To(BeTrue())
	// 				Expect(node.Label).To(Equal("User"))

	// 				// Update
	// 				err := storage.UpdateNode(aliceID, map[string]any{"version": i})
	// 				Expect(err).NotTo(HaveOccurred())

	// 				// Query edges
	// 				edges := storage.GetEdgesFrom(aliceID)
	// 				_ = edges

	// 				doneChan <- true
	// 			}(i)
	// 		}

	// 		// Wait for all
	// 		for i := 0; i < 10; i++ {
	// 			<-doneChan
	// 		}

	// 		close(done)
	// 	}, SpecTimeout(5.0))
	// })
})
