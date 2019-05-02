package router

import (
	"goblog/config"
	"goblog/controller/article"
	"goblog/controller/category"
	"goblog/controller/comment"
	"goblog/controller/common"
	"goblog/controller/user"
	"goblog/middleware"

	"github.com/gin-gonic/gin"
)

// Route 路由
func Route(router *gin.Engine) {
	apiPrefix := config.ServerConfig.APIPrefix

	api := router.Group(apiPrefix, middleware.RefreshTokenCookie)
	{
		api.GET("/article/list", article.List)
		api.GET("/article/detail/:id", article.Detail)
		api.POST("/article/save", middleware.AdminRequired, article.Save)
		api.DELETE("/article/delete/:id", middleware.AdminRequired, article.Delete)

		api.GET("/category/list", category.List)
		api.GET("/category/detail/:id", category.Detail)
		api.POST("/category/save", middleware.AdminRequired, category.Save)
		api.DELETE("/category/delete/:id", middleware.AdminRequired, category.Delete)

		api.GET("/comment/list/:articleID", comment.List)
		api.POST("/comment/save", comment.Save)
		api.DELETE("/comment/delete/:id", middleware.AdminRequired, comment.Delete)

		//		api.GET("/user/detail", user.Detail)
		api.POST("sign/in", user.Signin)
		api.POST("sign/up", user.Signup)
		api.POST("sign/out", user.Signout)

		api.POST("attachment/upload", middleware.AdminRequired, common.UploadHandler)
		api.DELETE("attachment/delete/:id", middleware.AdminRequired, common.DelateAttachment)
	}
}
