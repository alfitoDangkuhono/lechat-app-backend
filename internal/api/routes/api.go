package routes

import (
	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/middleware"
	"lechat-backend/internal/interfaces"

	"github.com/gin-gonic/gin"
)

func Routes(router *gin.Engine, authenticationController *controllers.AuthenticationController, jwt interfaces.IJwtInterface, chatControllers ...*controllers.ChatController) {
	api := router.Group("api/v1")

	api.POST("/registers", authenticationController.Register)
	api.POST("/sessions", authenticationController.Login)
	api.POST("/sessions/refresh", middleware.RefreshAuthentication(jwt), authenticationController.Refresh)

	authenticated := api.Group("")
	authenticated.Use(middleware.Authentication(jwt))
	authenticated.DELETE("/sessions", authenticationController.Logout)
	authenticated.GET("/sessions/check-session", authenticationController.CheckSession)
	authenticated.GET("/profile", authenticationController.Profile)
	authenticated.POST("/profile", authenticationController.UpdateProfile)
	if len(chatControllers) > 0 && chatControllers[0] != nil {
		chatController := chatControllers[0]
		authenticated.POST("/conversations", chatController.CreateConversation)
		authenticated.GET("/conversations", chatController.ListConversations)
		authenticated.GET("/conversations/:conversation_id/members", chatController.ListGroupMembers)
		authenticated.GET("/conversations/:conversation_id/messages", chatController.ListMessages)
		authenticated.POST("/conversations/:conversation_id/messages", chatController.SendMessage)
		authenticated.POST("/conversations/:conversation_id/delivered", chatController.MarkDelivered)
		authenticated.POST("/conversations/:conversation_id/read", chatController.MarkRead)
		authenticated.PATCH("/messages/:message_id", chatController.EditMessage)
		authenticated.DELETE("/messages/:message_id", chatController.DeleteMessage)
		authenticated.POST("/conversations/:conversation_id/invitations", chatController.InviteGroupMember)
		authenticated.DELETE("/conversations/:conversation_id/members/:member_id", chatController.RemoveMember)
		authenticated.GET("/group-invitations", chatController.ListGroupInvitations)
		authenticated.POST("/group-invitations/:invitation_id/accept", chatController.RespondToGroupInvitation)
		authenticated.DELETE("/group-invitations/:invitation_id", chatController.RespondToGroupInvitation)
	}
	// Add other protected endpoints to this group to require a valid Bearer token.
}
