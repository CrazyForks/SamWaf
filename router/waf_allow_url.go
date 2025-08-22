package router

import (
	"SamWaf/api"
	"github.com/gin-gonic/gin"
)

type AllowUrlRouter struct {
}

func (receiver *AllowUrlRouter) InitAllowUrlRouter(group *gin.RouterGroup) {
	AllowUrlRouterApi := api.APIGroupAPP.WafAllowUrlApi
	allowUrlRouter := group.Group("")
	allowUrlRouter.POST("/samwaf/wafhost/urlwhite/list", AllowUrlRouterApi.GetListApi)
	allowUrlRouter.GET("/samwaf/wafhost/urlwhite/detail", AllowUrlRouterApi.GetDetailApi)
	allowUrlRouter.POST("/samwaf/wafhost/urlwhite/add", AllowUrlRouterApi.AddApi)
	allowUrlRouter.GET("/samwaf/wafhost/urlwhite/del", AllowUrlRouterApi.DelAllowUrlApi)
	allowUrlRouter.POST("/samwaf/wafhost/urlwhite/edit", AllowUrlRouterApi.ModifyAllowUrlApi)
	// 新增批量删除路由
	allowUrlRouter.POST("/samwaf/wafhost/urlwhite/batchdel", AllowUrlRouterApi.BatchDelAllowUrlApi)
	// 新增全部删除路由
	allowUrlRouter.POST("/samwaf/wafhost/urlwhite/delall", AllowUrlRouterApi.DelAllAllowUrlApi)
}
