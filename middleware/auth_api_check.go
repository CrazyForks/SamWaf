package middleware

import (
	"SamWaf/common/zlog"
	"SamWaf/enums"
	"SamWaf/global"
	"SamWaf/model"
	"SamWaf/model/common/response"
	"SamWaf/service/waf_service"
	"fmt"
	"github.com/gin-gonic/gin"
	"strings"
	"time"
)

var (
	wafTokenInfoService = waf_service.WafTokenInfoServiceApp
	wafOtpService       = waf_service.WafOtpServiceApp
)

// Auth 鉴权中间件
func Auth() gin.HandlerFunc {
	innerName := "Auth"
	return func(c *gin.Context) {
		// 获取请求头中 token，实际是一个完整被签名过的 token；a complete, signed token
		tokenStr := ""
		if c.Request.RequestURI == "/samwaf/ws" {
			tokenStr = c.GetHeader("Sec-WebSocket-Protocol")
		} else if strings.HasPrefix(c.Request.RequestURI, "/samwaf/waflog/attack/download") {
			tokenStr = c.Query("X-Token")
		} else {
			tokenStr = c.GetHeader("X-Token")
		}
		if tokenStr == "" {
			zlog.Debug("无token")

			response.AuthFailWithMessage("鉴权失败", c)
			c.Abort()
			return
		} else {
			//检查是否存在
			isTokenExist := global.GCACHE_WAFCACHE.IsKeyExist(enums.CACHE_TOKEN + tokenStr)
			if !isTokenExist {
				response.AuthFailWithMessage("令牌过期", c)
				c.Abort()
				return
			} else {
				tokenInfo := global.GCACHE_WAFCACHE.Get(enums.CACHE_TOKEN + tokenStr).(model.TokenInfo)
				if tokenInfo.LoginIp != c.ClientIP() {
					zlog.Error(fmt.Sprintf("登录IP不一致，请求拒绝,原IP:%v 当前IP:%v", tokenInfo.LoginIp, c.ClientIP()))
					global.GCACHE_WAFCACHE.Remove(enums.CACHE_TOKEN + tokenStr)
					response.AuthFailWithMessage("本次登录IP和上次登录IP不一致需要重新登录", c)
					c.Abort()
					return
				} else {
					//刷新token时间
					if global.GWAF_RELEASE == "false" {
						tokenList := global.GCACHE_WAFCACHE.ListAvailableKeysWithPrefix(enums.CACHE_TOKEN)

						for _, duration := range tokenList {
							remainTime := fmt.Sprintf("%02d时%02d分", int(duration.Hours()), int(duration.Minutes())%60)
							zlog.Debug(fmt.Sprintf("%v 当前token有效缓存剩余时间 %v", innerName, remainTime))
						}
					}
					expireTime, err := global.GCACHE_WAFCACHE.GetExpireTime(enums.CACHE_TOKEN + tokenStr)
					if err == nil {
						remainingTime := time.Until(expireTime) // 计算剩余有效时间
						if remainingTime > 0 && remainingTime < 2*time.Minute {
							zlog.Debug(fmt.Sprintf("%v 当前token有效缓存剩余时间 %v  小于2分钟进行缓存可用时间延期处理", innerName, expireTime))
							global.GCACHE_WAFCACHE.SetWithTTlRenewTime(enums.CACHE_TOKEN+tokenStr, tokenInfo, time.Duration(global.GCONFIG_RECORD_TOKEN_EXPIRE_MINTUTES)*time.Minute)
						}
					}

					//检测是否强制2Fa绑定
					if global.GCONFIG_RECORD_FORCE_BIND_2FA == 1 && c.Request.RequestURI != "/samwaf/ws" && c.Request.RequestURI != "/samwaf/logout" {
						otpBean := wafOtpService.GetDetailByUserNameApi(tokenInfo.LoginAccount)
						if otpBean.UserName == "" {
							//需要强制跳转2fa绑定界面
							response.NeedBind2FAWithMessage("系统已开启强制 【双因素认证】 ，请进行绑定", c)
							c.Abort()
							return
						}
					}
				}
			}
		}

		// 这里执行路由 HandlerFunc
		c.Next()
	}
}
