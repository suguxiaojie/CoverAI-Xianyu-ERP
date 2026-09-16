package server

import (
	"github.com/go-chi/chi/v5"

	"xianyu-go/internal/auth"
)

// mountVersionedKnowledgeRoutes 挂载人工知识库与离线检索调试的 `/api/v1` 路由。
func (server *Server) mountVersionedKnowledgeRoutes(router chi.Router) {
	router.Group(func(router chi.Router) {
		router.Use(server.Auth.Middleware)
		router.Use(auth.RequireAuth)
		router.Get("/api/v1/knowledge-bases", server.listKnowledgeBases)
		router.Post("/api/v1/knowledge-bases", server.createKnowledgeBase)
		router.Get("/api/v1/knowledge-bases/{knowledge_base_id}", server.getKnowledgeBase)
		router.Put("/api/v1/knowledge-bases/{knowledge_base_id}", server.updateKnowledgeBase)
		router.Delete("/api/v1/knowledge-bases/{knowledge_base_id}", server.deleteKnowledgeBase)
		router.Put("/api/v1/knowledge-bases/{knowledge_base_id}/status", server.setKnowledgeBaseStatus)
		router.Get("/api/v1/knowledge-bases/{knowledge_base_id}/entries", server.listKnowledgeEntries)
		router.Post("/api/v1/knowledge-bases/{knowledge_base_id}/entries", server.createKnowledgeEntry)
		router.Put("/api/v1/knowledge-bases/{knowledge_base_id}/entries/{entry_type}/{entry_id}", server.updateKnowledgeEntry)
		router.Delete("/api/v1/knowledge-bases/{knowledge_base_id}/entries/{entry_type}/{entry_id}", server.deleteKnowledgeEntry)
		router.Put("/api/v1/knowledge-bases/{knowledge_base_id}/entries/{entry_type}/{entry_id}/review", server.reviewKnowledgeEntry)
		router.Put("/api/v1/knowledge-bases/{knowledge_base_id}/entries/{entry_type}/{entry_id}/enabled", server.setKnowledgeEntryEnabled)
		router.Get("/api/v1/knowledge-bases/{knowledge_base_id}/entries/faq/{faq_id}/aliases", server.listKnowledgeFAQAliases)
		router.Post("/api/v1/knowledge-bases/{knowledge_base_id}/entries/faq/{faq_id}/aliases", server.createKnowledgeFAQAlias)
		router.Delete("/api/v1/knowledge-bases/{knowledge_base_id}/entries/faq/{faq_id}/aliases/{alias_id}", server.deleteKnowledgeFAQAlias)
		router.Get("/api/v1/knowledge-bases/{knowledge_base_id}/entries/faq/{faq_id}/alias-suggestions", server.suggestKnowledgeFAQAliases)
		router.Post("/api/v1/knowledge-retrieve", server.retrieveKnowledge)
	})
}
