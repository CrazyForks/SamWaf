package wafenginecore

import (
	"SamWaf/global"
	"SamWaf/innerbean"
	"SamWaf/libinjection-go"
	"SamWaf/model"
	"SamWaf/model/wafenginmodel"
	"SamWaf/plugin"
	"SamWaf/utils"
	"SamWaf/utils/zlog"
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"github.com/satori/go.uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"golang.org/x/time/rate"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type WafEngine struct {
	//主机情况
	HostTarget map[string]*wafenginmodel.HostSafe
	//主机和code的关系
	HostCode     map[string]string
	ServerOnline map[int]innerbean.ServerRunTime

	//所有证书情况 对应端口 可能多个端口都是https 443，或者其他非标准端口也要实现https证书
	AllCertificate map[int]map[string]*tls.Certificate
	EsHelper       utils.EsHelper

	EngineCurrentStatus int // 当前waf引擎状态
}

func (waf *WafEngine) Error() string {
	fs := "HTTP: %d, HostCode: %d, Message: %s"
	return fmt.Sprintf(fs)
}
func (waf *WafEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if !strings.Contains(host, ":") {
		host = host + ":80"
	}
	defer func() {
		e := recover()
		if e != nil { // 捕获该协程的panic 111111
			fmt.Println("11recover ", e)
		}
	}()
	// 检查域名是否已经注册
	if target, ok := waf.HostTarget[host]; ok {

		// 获取请求报文的内容长度
		contentLength := r.ContentLength

		//server_online[8081].Svr.Close()
		var bodyByte []byte

		// 拷贝一份request的Body ,控制不记录大文件的情况 ，先写死的
		if r.Body != nil && r.Body != http.NoBody && contentLength < (global.GCONFIG_RECORD_MAX_BODY_LENGTH) {
			bodyByte, _ = io.ReadAll(r.Body)
			// 把刚刚读出来的再写进去，不然后面解析表单数据就解析不到了
			r.Body = io.NopCloser(bytes.NewBuffer(bodyByte))
		}
		cookies, _ := json.Marshal(r.Cookies())
		header, _ := json.Marshal(r.Header)
		// 取出客户IP
		ipAndPort := strings.Split(r.RemoteAddr, ":")
		region := utils.GetCountry(ipAndPort[0])
		currentDay, _ := strconv.Atoi(time.Now().Format("20060102"))
		weblogbean := innerbean.WebLog{
			HOST:           host,
			URL:            r.RequestURI,
			REFERER:        r.Referer(),
			USER_AGENT:     r.UserAgent(),
			METHOD:         r.Method,
			HEADER:         string(header),
			COUNTRY:        region[0],
			PROVINCE:       region[2],
			CITY:           region[3],
			SRC_IP:         ipAndPort[0],
			SRC_PORT:       ipAndPort[1],
			CREATE_TIME:    time.Now().Format("2006-01-02 15:04:05"),
			CONTENT_LENGTH: contentLength,
			COOKIES:        string(cookies),
			BODY:           string(bodyByte),
			REQ_UUID:       uuid.NewV4().String(),
			USER_CODE:      global.GWAF_USER_CODE,
			HOST_CODE:      waf.HostTarget[host].Host.Code,
			TenantId:       global.GWAF_TENANT_ID,
			RULE:           "",
			ACTION:         "通过",
			Day:            currentDay,
		}

		if waf.HostTarget[host].Host.GUARD_STATUS == 1 {
			var jumpGuardFlag = false
			//ip白名单策略（局部）
			if waf.HostTarget[host].IPWhiteLists != nil {
				for i := 0; i < len(waf.HostTarget[host].IPWhiteLists); i++ {
					if waf.HostTarget[host].IPWhiteLists[i].Ip == weblogbean.SRC_IP {
						jumpGuardFlag = true
						break
					}
				}
			}
			//ip白名单策略（全局）
			if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Host.GUARD_STATUS == 1 && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].IPWhiteLists != nil {
				for i := 0; i < len(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].IPWhiteLists); i++ {
					if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].IPWhiteLists[i].Ip == weblogbean.SRC_IP {
						jumpGuardFlag = true
						break
					}
				}
			}
			//url白名单策略（局部）
			if waf.HostTarget[host].UrlWhiteLists != nil {
				for i := 0; i < len(waf.HostTarget[host].UrlWhiteLists); i++ {
					if (waf.HostTarget[host].UrlWhiteLists[i].CompareType == "等于" && waf.HostTarget[host].UrlWhiteLists[i].Url == weblogbean.URL) ||
						(waf.HostTarget[host].UrlWhiteLists[i].CompareType == "前缀匹配" && strings.HasPrefix(weblogbean.URL, waf.HostTarget[host].UrlWhiteLists[i].Url)) ||
						(waf.HostTarget[host].UrlWhiteLists[i].CompareType == "后缀匹配" && strings.HasSuffix(weblogbean.URL, waf.HostTarget[host].UrlWhiteLists[i].Url)) ||
						(waf.HostTarget[host].UrlWhiteLists[i].CompareType == "包含匹配" && strings.Contains(weblogbean.URL, waf.HostTarget[host].UrlWhiteLists[i].Url)) {
						jumpGuardFlag = true
						break
					}
				}
			}
			//url白名单策略（全局）
			if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Host.GUARD_STATUS == 1 && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists != nil {
				for i := 0; i < len(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists); i++ {
					if (waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].CompareType == "等于" && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].Url == weblogbean.URL) ||
						(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].CompareType == "前缀匹配" && strings.HasPrefix(weblogbean.URL, waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].Url)) ||
						(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].CompareType == "后缀匹配" && strings.HasSuffix(weblogbean.URL, waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].Url)) ||
						(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].CompareType == "包含匹配" && strings.Contains(weblogbean.URL, waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlWhiteLists[i].Url)) {
						jumpGuardFlag = true
						break
					}
				}
			}
			//ip黑名单策略  （局部）
			if waf.HostTarget[host].IPBlockLists != nil {
				for i := 0; i < len(waf.HostTarget[host].IPBlockLists); i++ {
					if waf.HostTarget[host].IPBlockLists[i].Ip == weblogbean.SRC_IP {
						EchoErrorInfo(w, r, weblogbean, "IP黑名单", "您的访问被阻止了IP限制")
						return
					}
				}
			}
			//ip黑名单策略（全局）
			if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Host.GUARD_STATUS == 1 && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].IPBlockLists != nil {
				for i := 0; i < len(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].IPBlockLists); i++ {
					if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].IPBlockLists[i].Ip == weblogbean.SRC_IP {
						EchoErrorInfo(w, r, weblogbean, "【全局】IP黑名单", "您的访问被阻止了IP限制")
						return
					}
				}
			}

			//url黑名单策略-(局部) （待优化性能）
			if waf.HostTarget[host].UrlBlockLists != nil {
				for i := 0; i < len(waf.HostTarget[host].UrlBlockLists); i++ {
					if waf.HostTarget[host].UrlBlockLists[i].Url == weblogbean.URL {
						EchoErrorInfo(w, r, weblogbean, "URL黑名单", "您的访问被阻止了URL限制")
						return
					}
				}
			}
			//url黑名单策略-(全局) （待优化性能）
			if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Host.GUARD_STATUS == 1 && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlBlockLists != nil {
				for i := 0; i < len(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlBlockLists); i++ {
					if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].UrlBlockLists[i].Url == weblogbean.URL {
						EchoErrorInfo(w, r, weblogbean, "【全局】URL黑名单", "您的访问被阻止了URL限制")
						return
					}
				}
			}

			if jumpGuardFlag == false {
				var sqlFlag = false
				//检测sql注入
				sqliResult, _ := libinjection.IsSQLi(weblogbean.URL)
				if sqliResult {
					sqlFlag = true
				}
				sqliResult, _ = libinjection.IsSQLi(weblogbean.BODY)
				if sqliResult {
					sqlFlag = true
				}
				if sqlFlag == true {
					EchoErrorInfo(w, r, weblogbean, "SQL注入", "请正确访问")
					return
				}
				//检测xss注入
				var xssFlag = false
				sqlixssResult := libinjection.IsXSS(weblogbean.URL)
				if sqlixssResult == true {
					xssFlag = true
				}
				if xssFlag == true {
					EchoErrorInfo(w, r, weblogbean, "XSS跨站注入", "请正确访问")
					return
				}
				//检测xss
				// cc 防护 (局部检测 )
				if waf.HostTarget[host].PluginIpRateLimiter != nil {
					limiter := waf.HostTarget[host].PluginIpRateLimiter.GetLimiter(weblogbean.SRC_IP)
					if !limiter.Allow() {
						fmt.Println("超量了")
						EchoErrorInfo(w, r, weblogbean, "触发IP频次访问限制1", "您的访问被阻止超量了1")
						return
					}
				}
				// cc 防护 （全局检测 ）
				if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Host.GUARD_STATUS == 1 && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].PluginIpRateLimiter != nil {
					limiter := waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].PluginIpRateLimiter.GetLimiter(weblogbean.SRC_IP)
					if !limiter.Allow() {
						fmt.Println("超量了")
						EchoErrorInfo(w, r, weblogbean, "【全局】触发IP频次访问限制", "您的访问被阻止超量了")
						return
					}
				}

				//规则判断 （局部）
				if waf.HostTarget[host].Rule != nil {
					if waf.HostTarget[host].Rule.KnowledgeBase != nil {
						ruleMatchs, err := waf.HostTarget[host].Rule.Match("MF", &weblogbean)
						if err == nil {
							if len(ruleMatchs) > 0 {
								rulestr := ""
								for _, v := range ruleMatchs {
									rulestr = rulestr + v.RuleDescription + ","
								}
								w.Header().Set("WAF", "SAMWAF DROP")
								EchoErrorInfo(w, r, weblogbean, rulestr, "您的访问被阻止触发规则")
								return
							}
						} else {
							zlog.Debug("规则 ", err)
						}
					}
				}
				//规则判断 （全局网站）
				if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Host.GUARD_STATUS == 1 && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Rule != nil {
					if waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Rule.KnowledgeBase != nil {
						ruleMatchs, err := waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].Rule.Match("MF", &weblogbean)
						if err == nil {
							if len(ruleMatchs) > 0 {
								rulestr := ""
								for _, v := range ruleMatchs {
									rulestr = rulestr + v.RuleDescription + ","
								}
								w.Header().Set("WAF", "SAMWAF DROP")
								EchoErrorInfo(w, r, weblogbean, "【全局】"+rulestr, "您的访问被阻止触发规则")
								return
							}
						} else {
							zlog.Debug("规则 ", err)
						}
					}

				}

			}

		}

		remoteUrl, err := url.Parse(target.TargetHost)
		if err != nil {
			zlog.Debug("target parse fail:", zap.Any("", err))
			return
		}

		// 直接从缓存取出
		if waf.HostTarget[host].RevProxy != nil {
			waf.HostTarget[host].RevProxy.ServeHTTP(w, r)
		} else {
			proxy := httputil.NewSingleHostReverseProxy(remoteUrl)
			proxy.ModifyResponse = waf.modifyResponse()
			proxy.ErrorHandler = errorHandler()
			waf.HostTarget[host].RevProxy = proxy // 放入缓存
			proxy.ServeHTTP(w, r)
		}
		return
	} else {
		resBytes := []byte("403: Host forbidden " + host)
		w.Write(resBytes)
		// 获取请求报文的内容长度
		contentLength := r.ContentLength

		//server_online[8081].Svr.Close()
		var bodyByte []byte

		// 拷贝一份request的Body ,控制不记录大文件的情况 ，先写死的
		if r.Body != nil && r.Body != http.NoBody && contentLength < (global.GCONFIG_RECORD_MAX_BODY_LENGTH) {
			bodyByte, _ = io.ReadAll(r.Body)
			// 把刚刚读出来的再写进去，不然后面解析表单数据就解析不到了
			r.Body = io.NopCloser(bytes.NewBuffer(bodyByte))
		}
		cookies, _ := json.Marshal(r.Cookies())
		header, _ := json.Marshal(r.Header)
		// 取出客户IP
		ipAndPort := strings.Split(r.RemoteAddr, ":")
		region := utils.GetCountry(ipAndPort[0])
		currentDay, _ := strconv.Atoi(time.Now().Format("20060102"))
		weblogbean := innerbean.WebLog{
			HOST:           r.Host,
			URL:            r.RequestURI,
			REFERER:        r.Referer(),
			USER_AGENT:     r.UserAgent(),
			METHOD:         r.Method,
			HEADER:         string(header),
			COUNTRY:        region[0],
			PROVINCE:       region[2],
			CITY:           region[3],
			SRC_IP:         ipAndPort[0],
			SRC_PORT:       ipAndPort[1],
			CREATE_TIME:    time.Now().Format("2006-01-02 15:04:05"),
			CONTENT_LENGTH: contentLength,
			COOKIES:        string(cookies),
			BODY:           string(bodyByte),
			REQ_UUID:       uuid.NewV4().String(),
			USER_CODE:      global.GWAF_USER_CODE,
			HOST_CODE:      "",
			TenantId:       global.GWAF_TENANT_ID,
			RULE:           "",
			ACTION:         "通过",
			Day:            currentDay,
			STATUS:         "禁止访问",
			STATUS_CODE:    403,
		}

		//记录响应body
		weblogbean.RES_BODY = string(resBytes)
		weblogbean.ACTION = "禁止"
		global.GQEQUE_LOG_DB.PushBack(weblogbean)
	}
}
func EchoErrorInfo(w http.ResponseWriter, r *http.Request, weblogbean innerbean.WebLog, ruleName string, blockInfo string) {
	//通知信息
	noticeStr := fmt.Sprintf("网站域名:%s 访问IP:%s 归属地区：%s  规则：%s 阻止信息：%s", weblogbean.HOST, weblogbean.SRC_IP, utils.GetCountry(weblogbean.SRC_IP), ruleName, blockInfo)

	zlog.Debug(noticeStr)
	//发送微信推送消息
	global.GQEQUE_MESSAGE_DB.PushBack(innerbean.RuleMessageInfo{
		BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "命中保护规则", Server: global.GWAF_CUSTOM_SERVER_NAME},
		Domain:          weblogbean.HOST,
		RuleInfo:        ruleName,
		Ip:              fmt.Sprintf("%s (%s)", weblogbean.SRC_IP, utils.GetCountry(weblogbean.SRC_IP)),
	})

	resBytes := []byte("<html><head><title>您的访问被阻止</title></head><body><center><h1>" + blockInfo + "</h1> <br> 访问识别码：<h3>" + weblogbean.REQ_UUID + "</h3></center></body> </html>")
	w.Write(resBytes)

	//记录响应body
	weblogbean.RES_BODY = string(resBytes)
	weblogbean.RULE = ruleName
	weblogbean.ACTION = "阻止"
	weblogbean.STATUS = "阻止访问"
	weblogbean.STATUS_CODE = 403
	global.GQEQUE_LOG_DB.PushBack(weblogbean)

	zlog.Debug(ruleName)
}
func errorHandler() func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, req *http.Request, err error) {
		zlog.Debug("Got error  response:", zap.Any("err", err))
		return
	}
}

func (waf *WafEngine) modifyResponse() func(*http.Response) error {
	return func(resp *http.Response) error {
		resp.Header.Set("WAF", "SamWAF")
		resp.Header.Set("Server", "SamWAF")
		r := resp.Request
		//开始准备req数据
		//获取请求报文的内容长度
		contentLength := r.ContentLength

		//server_online[8081].Svr.Close()
		var bodyByte []byte

		// 拷贝一份request的Body ,控制不记录大文件的情况 ，先写死的
		if r.Body != nil && r.Body != http.NoBody && contentLength < (global.GCONFIG_RECORD_MAX_BODY_LENGTH) {
			bodyByte, _ = io.ReadAll(r.Body)
			// 把刚刚读出来的再写进去，不然后面解析表单数据就解析不到了
			r.Body = io.NopCloser(bytes.NewBuffer(bodyByte))
		}
		cookies, _ := json.Marshal(r.Cookies())
		header, _ := json.Marshal(r.Header)
		// 取出客户IP
		ipAndPort := strings.Split(r.RemoteAddr, ":")
		region := utils.GetCountry(ipAndPort[0])
		currentDay, _ := strconv.Atoi(time.Now().Format("20060102"))
		weblogbean := innerbean.WebLog{
			HOST:           r.Host,
			URL:            r.RequestURI,
			REFERER:        r.Referer(),
			USER_AGENT:     r.UserAgent(),
			METHOD:         r.Method,
			HEADER:         string(header),
			COUNTRY:        region[0],
			PROVINCE:       region[2],
			CITY:           region[3],
			SRC_IP:         ipAndPort[0],
			SRC_PORT:       ipAndPort[1],
			CREATE_TIME:    time.Now().Format("2006-01-02 15:04:05"),
			CONTENT_LENGTH: contentLength,
			COOKIES:        string(cookies),
			BODY:           string(bodyByte),
			REQ_UUID:       uuid.NewV4().String(),
			USER_CODE:      global.GWAF_USER_CODE,
			HOST_CODE:      "",
			TenantId:       global.GWAF_TENANT_ID,
			RULE:           "",
			ACTION:         "通过",
			Day:            currentDay,
			STATUS:         resp.Status,
			STATUS_CODE:    resp.StatusCode,
		}

		host := resp.Request.Host
		if !strings.Contains(host, ":") {
			host = host + ":80"
		}
		if waf.HostTarget[host] != nil {
			weblogbean.HOST_CODE = waf.HostTarget[host].Host.Code
		}
		zlog.Debug("%s %s", resp.Request.Host, resp.Request.RequestURI)
		ldpFlag := false
		//隐私保护（局部）
		for i := 0; i < len(waf.HostTarget[host].LdpUrlLists); i++ {
			if (waf.HostTarget[host].LdpUrlLists[i].CompareType == "等于" && waf.HostTarget[host].LdpUrlLists[i].Url == resp.Request.RequestURI) ||
				(waf.HostTarget[host].LdpUrlLists[i].CompareType == "前缀匹配" && strings.HasPrefix(resp.Request.RequestURI, waf.HostTarget[host].LdpUrlLists[i].Url)) ||
				(waf.HostTarget[host].LdpUrlLists[i].CompareType == "后缀匹配" && strings.HasSuffix(resp.Request.RequestURI, waf.HostTarget[host].LdpUrlLists[i].Url)) ||
				(waf.HostTarget[host].LdpUrlLists[i].CompareType == "包含匹配" && strings.Contains(resp.Request.RequestURI, waf.HostTarget[host].LdpUrlLists[i].Url)) {

				ldpFlag = true
				break
			}
		}
		//隐私保护（局部）
		for i := 0; i < len(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists); i++ {
			if (waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].CompareType == "等于" && waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].Url == resp.Request.RequestURI) ||
				(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].CompareType == "前缀匹配" && strings.HasPrefix(resp.Request.RequestURI, waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].Url)) ||
				(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].CompareType == "后缀匹配" && strings.HasSuffix(resp.Request.RequestURI, waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].Url)) ||
				(waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].CompareType == "包含匹配" && strings.Contains(resp.Request.RequestURI, waf.HostTarget[global.GWAF_GLOBAL_HOST_NAME].LdpUrlLists[i].Url)) {

				ldpFlag = true
				break
			}
		}
		if ldpFlag == true {
			//编码转换，自动检测网页编码
			convertCntReader, _ := switchContentEncoding(resp)
			bodyReader := bufio.NewReader(convertCntReader)
			charset := determinePageEncoding(bodyReader)

			reader := transform.NewReader(bodyReader, charset.NewDecoder())
			oldBytes, err := io.ReadAll(reader)
			if err != nil {
				zlog.Info("error", err)
				return nil
			}

			// body 追加内容
			//zlog.Debug(string(oldBytes))
			//newPayload := []byte("" +  strings.Replace(string(oldBytes), "青岛", "**", -1))
			newPayload := []byte("" + utils.DeSenText(string(oldBytes)))

			finalBytes, _ := switchReplyContentEncoding(resp, newPayload) //utils.GZipEncode(newPayload)
			//zlog.Debug("转换完",string(finalBytes))
			resp.Body = io.NopCloser(bytes.NewBuffer(finalBytes))

			// head 修改追加内容
			resp.ContentLength = int64(len(newPayload))
			resp.Header.Set("Content-Length", strconv.FormatInt(int64(len(finalBytes)), 10))
		}
		//返回内容的类型
		respContentType := resp.Header.Get("Content-Type")
		contentType := ""
		//JS情况
		if respContentType == "application/javascript" {
			contentType = "js"
		}
		//CSS情况
		if respContentType == "text/css" {
			contentType = "css"
		}
		//图片情况
		if respContentType == "image/jpeg" || respContentType == "image/png" || respContentType == "image/png; charset=utf-8" ||
			respContentType == "image/gif" || respContentType == "image/x-icon" {
			contentType = "img"
		}
		//记录静态日志
		isStaticAssist := false
		switch contentType {
		case "js":
			isStaticAssist = true
			break
		case "css":
			isStaticAssist = true
			break
		case "img":
			isStaticAssist = true
			break
		default:
			/*weblogbean.ACTION = "放行"
			global.GQEQUE_LOG_DB.PushBack(weblogbean)*/
		}

		//记录响应body
		if resp.Body != nil && resp.Body != http.NoBody && global.GCONFIG_RECORD_RESP == 1 {
			//编码转换，自动检测网页编码
			convertCntReader, _ := switchContentEncoding(resp)
			bodyReader := bufio.NewReader(convertCntReader)
			charset := determinePageEncoding(bodyReader)

			reader := transform.NewReader(bodyReader, charset.NewDecoder())
			resbodyByte, err := io.ReadAll(reader)
			if err != nil {
				zlog.Info("error", err)
				return nil
			}
			// 把刚刚读出来的再写进去，不然后面解析表单数据就解析不到了
			resp.Body = io.NopCloser(bytes.NewBuffer(resbodyByte))
			if resp.ContentLength < global.GCONFIG_RECORD_MAX_RES_BODY_LENGTH {
				weblogbean.RES_BODY = string(resbodyByte)
			}

		}

		//TODO 如果是指定URL 或者 IP 不记录日志
		if !isStaticAssist && !strings.Contains(weblogbean.URL, "index.php/lttshop/task_scheduling/") {
			weblogbean.ACTION = "放行"
			weblogbean.STATUS = resp.Status
			weblogbean.STATUS_CODE = resp.StatusCode
			global.GQEQUE_LOG_DB.PushBack(weblogbean)
		}

		return nil
	}
}

// 返回是否需要进行压缩

func switchReplyContentEncoding(res *http.Response, inputBytes []byte) (respBytes []byte, err error) {

	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		respBytes, err = utils.GZipEncode(inputBytes)
	case "deflate":
		respBytes, err = utils.DeflateEncode(inputBytes)
	default:
		respBytes = inputBytes
	}
	return
}

// 检测返回的body是否经过压缩，并返回解压的内容
func switchContentEncoding(res *http.Response) (bodyReader io.Reader, err error) {
	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		bodyReader, err = gzip.NewReader(res.Body)
	case "deflate":
		bodyReader = flate.NewReader(res.Body)
	default:
		bodyReader = res.Body
	}
	return
}

// 检测html页面编码
func determinePageEncoding(r *bufio.Reader) encoding.Encoding {
	//使用peek读取十分关键，只是偷看一下，不会移动读取位置，否则其他地方就没法读取了
	bytes, err := r.Peek(1024)
	if err != nil {
		log.Printf("Fetcher error: %v\n", err)
		return unicode.UTF8
	}
	e, _, _ := charset.DetermineEncoding(bytes, "")
	return e
}
func (waf *WafEngine) Start_WAF() {
	config := viper.New()
	config.AddConfigPath(utils.GetCurrentDir() + "/conf/") // 文件所在目录
	config.SetConfigName("config")                         // 文件名
	config.SetConfigType("yml")                            // 文件类型
	waf.EngineCurrentStatus = 1
	if err := config.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			zlog.Error("找不到配置文件..")
		} else {
			zlog.Error("配置文件出错..")
		}
	}

	global.GWAF_USER_CODE = config.GetString("user_code") // 读取配置
	global.GWAF_TENANT_ID = global.GWAF_USER_CODE
	global.GWAF_LOCAL_SERVER_PORT = config.GetInt("local_port")             //读取本地端口
	global.GWAF_CUSTOM_SERVER_NAME = config.GetString("custom_server_name") //本地服务器其定义名称
	zlog.Debug(" load ini: ", global.GWAF_USER_CODE)

	var hosts []model.Hosts
	//是否有初始化全局保护
	global.GWAF_LOCAL_DB.Where("global_host = ?", 1).Find(&hosts)
	if hosts != nil && len(hosts) == 0 {
		//初始化全局保护
		var wafGlobalHost = &model.Hosts{
			USER_CODE:     global.GWAF_USER_CODE,
			Tenant_id:     global.GWAF_TENANT_ID,
			Code:          uuid.NewV4().String(),
			Host:          "全局网站",
			Port:          0,
			Ssl:           0,
			GUARD_STATUS:  0,
			REMOTE_SYSTEM: "",
			REMOTE_APP:    "",
			Remote_host:   "",
			Remote_port:   0,
			Certfile:      "",
			Keyfile:       "",
			REMARKS:       "",
			CREATE_TIME:   time.Now(),
			UPDATE_TIME:   time.Now(),
			GLOBAL_HOST:   1,
		}
		global.GWAF_LOCAL_DB.Create(wafGlobalHost)
	}
	//重新查询
	global.GWAF_LOCAL_DB.Find(&hosts)

	//初始化步骤[加载ip数据库]
	var dbPath = utils.GetCurrentDir() + "/data/ip2region.xdb"
	// 1、从 dbPath 加载整个 xdb 到内存
	cBuff, err := xdb.LoadContentFromFile(dbPath)
	if err != nil {
		zlog.Info("加载ip库错误")
		zlog.Debug("failed to load content from `%s`: %s\n", dbPath, err)
		return
	}
	global.GCACHE_IP_CBUFF = cBuff

	//第一步 检测合法性并加入到全局

	for i := 0; i < len(hosts); i++ {
		//检测https
		if hosts[i].Ssl == 1 {
			cert, err := tls.X509KeyPair([]byte(hosts[i].Certfile), []byte(hosts[i].Keyfile))
			if err != nil {
				zlog.Warn("Cannot find %s cert & key file. Error is: %s\n", hosts[i].Host, err)
				continue

			}
			//all_certificate[hosts[i].Port][hosts[i].Host] = &cert
			mm, ok := waf.AllCertificate[hosts[i].Port] //[hosts[i].Host]
			if !ok {
				mm = make(map[string]*tls.Certificate)
				waf.AllCertificate[hosts[i].Port] = mm
			}
			waf.AllCertificate[hosts[i].Port][hosts[i].Host] = &cert
		}
		_, ok := waf.ServerOnline[hosts[i].Port]
		if ok == false && hosts[i].GLOBAL_HOST == 0 {
			waf.ServerOnline[hosts[i].Port] = innerbean.ServerRunTime{
				ServerType: utils.GetServerByHosts(hosts[i]),
				Port:       hosts[i].Port,
				Status:     0,
			}
		}

		//加载主机对于的规则
		ruleHelper := &utils.RuleHelper{}
		ruleHelper.InitRuleEngine()
		//查询规则
		var vcnt int
		global.GWAF_LOCAL_DB.Model(&model.Rules{}).Where("host_code = ? and rule_status<>999",
			hosts[i].Code).Select("sum(rule_version) as vcnt").Row().Scan(&vcnt)
		zlog.Debug("主机host" + hosts[i].Code + " 版本" + strconv.Itoa(vcnt))
		var ruleconfigs []model.Rules
		if vcnt > 0 {
			global.GWAF_LOCAL_DB.Where("host_code = ?and rule_status<>999", hosts[i].Code).Find(&ruleconfigs)
			ruleHelper.LoadRules(ruleconfigs)
		}
		//查询ip限流(应该针对一个网址只有一个)
		var anticcBean model.AntiCC

		global.GWAF_LOCAL_DB.Where("host_code=? ", hosts[i].Code).Limit(1).Find(&anticcBean)

		//初始化插件-ip计数器
		var pluginIpRateLimiter *plugin.IPRateLimiter
		if anticcBean.Id != "" {
			pluginIpRateLimiter = plugin.NewIPRateLimiter(rate.Limit(anticcBean.Rate), anticcBean.Limit)
		}

		//查询ip白名单
		var ipwhitelist []model.IPWhiteList
		global.GWAF_LOCAL_DB.Where("host_code=? ", hosts[i].Code).Find(&ipwhitelist)

		//查询url白名单
		var urlwhitelist []model.URLWhiteList
		global.GWAF_LOCAL_DB.Where("host_code=? ", hosts[i].Code).Find(&urlwhitelist)

		//查询ip黑名单
		var ipblocklist []model.IPBlockList
		global.GWAF_LOCAL_DB.Where("host_code=? ", hosts[i].Code).Find(&ipblocklist)

		//查询url白名单
		var urlblocklist []model.URLBlockList
		global.GWAF_LOCAL_DB.Where("host_code=? ", hosts[i].Code).Find(&urlblocklist)

		//查询url隐私保护
		var ldpurls []model.LDPUrl
		global.GWAF_LOCAL_DB.Where("host_code=? ", hosts[i].Code).Find(&ldpurls)

		//初始化主机host
		hostsafe := &wafenginmodel.HostSafe{
			RevProxy:            nil,
			Rule:                ruleHelper,
			TargetHost:          hosts[i].Remote_host + ":" + strconv.Itoa(hosts[i].Remote_port),
			RuleData:            ruleconfigs,
			RuleVersionSum:      vcnt,
			Host:                hosts[i],
			PluginIpRateLimiter: pluginIpRateLimiter,
			IPWhiteLists:        ipwhitelist,
			UrlWhiteLists:       urlwhitelist,
			LdpUrlLists:         ldpurls,
			IPBlockLists:        ipblocklist,
			UrlBlockLists:       urlblocklist,
		}
		//赋值到白名单里面
		waf.HostTarget[hosts[i].Host+":"+strconv.Itoa(hosts[i].Port)] = hostsafe
		//赋值到对照表里面
		waf.HostCode[hosts[i].Code] = hosts[i].Host + ":" + strconv.Itoa(hosts[i].Port)

	}

	wafSysLog := &model.WafSysLog{
		Id:         uuid.NewV4().String(),
		UserCode:   global.GWAF_USER_CODE,
		TenantId:   global.GWAF_TENANT_ID,
		OpType:     "信息",
		OpContent:  "WAF启动",
		CreateTime: time.Now(),
	}
	global.GQEQUE_LOG_DB.PushBack(wafSysLog)

	for _, v := range waf.ServerOnline {
		go func(innruntime innerbean.ServerRunTime) {

			if (innruntime.ServerType) == "https" {

				defer func() {
					e := recover()
					if e != nil { // 捕获该协程的panic 111111
						zlog.Warn("https recover ", e)
					}
				}()
				svr := &http.Server{
					Addr:    ":" + strconv.Itoa(innruntime.Port),
					Handler: waf,
					TLSConfig: &tls.Config{
						NameToCertificate: make(map[string]*tls.Certificate, 0),
					},
				}
				serclone := waf.ServerOnline[innruntime.Port]
				serclone.Svr = svr
				waf.ServerOnline[innruntime.Port] = serclone

				svr.TLSConfig.NameToCertificate = waf.AllCertificate[innruntime.Port]
				svr.TLSConfig.GetCertificate = func(clientInfo *tls.ClientHelloInfo) (*tls.Certificate, error) {
					if x509Cert, ok := svr.TLSConfig.NameToCertificate[clientInfo.ServerName]; ok {
						return x509Cert, nil
					}
					return nil, errors.New("config error")
				}
				zlog.Info("启动HTTPS 服务器" + strconv.Itoa(innruntime.Port))
				err = svr.ListenAndServeTLS("", "")
				if err == http.ErrServerClosed {
					zlog.Error("[HTTPServer] https server has been close, cause:[%v]", err)
				} else {
					//TODO 记录如果https 端口被占用的情况 记录日志 且应该推送websocket
					wafSysLog := model.WafSysLog{
						Id:         uuid.NewV4().String(),
						UserCode:   global.GWAF_USER_CODE,
						TenantId:   global.GWAF_TENANT_ID,
						OpType:     "系统运行错误",
						OpContent:  "HTTPS端口被占用: " + strconv.Itoa(innruntime.Port) + ",请检查",
						CreateTime: time.Time{},
					}
					global.GQEQUE_LOG_DB.PushBack(wafSysLog)
					zlog.Error("[HTTPServer] https server start fail, cause:[%v]", err)
				}
				zlog.Info("server https shutdown")

			} else {
				defer func() {
					e := recover()
					if e != nil { // 捕获该协程的panic 111111
						zlog.Warn("http recover ", e)
					}
				}()
				svr := &http.Server{
					Addr:    ":" + strconv.Itoa(innruntime.Port),
					Handler: waf,
				}
				serclone := waf.ServerOnline[innruntime.Port]
				serclone.Svr = svr
				waf.ServerOnline[innruntime.Port] = serclone

				zlog.Info("启动HTTP 服务器" + strconv.Itoa(innruntime.Port))
				err = svr.ListenAndServe()
				if err == http.ErrServerClosed {
					zlog.Warn("[HTTPServer] http server has been close, cause:[%v]", err)
				} else {
					//TODO 记录如果http 端口被占用的情况 记录日志 且应该推送websocket
					wafSysLog := model.WafSysLog{
						Id:         uuid.NewV4().String(),
						UserCode:   global.GWAF_USER_CODE,
						TenantId:   global.GWAF_TENANT_ID,
						OpType:     "系统运行错误",
						OpContent:  "HTTP端口被占用: " + strconv.Itoa(innruntime.Port) + ",请检查",
						CreateTime: time.Time{},
					}
					global.GQEQUE_LOG_DB.PushBack(wafSysLog)
					zlog.Error("[HTTPServer] http server start fail, cause:[%v]", err)
				}
				zlog.Info("server  http shutdown")
			}

		}(v)

	}
}

// 关闭waf
func (waf *WafEngine) CLoseWAF() {
	defer func() {
		e := recover()
		if e != nil { // 捕获该协程的panic 111111
			zlog.Debug("关闭 recover ", e)
		}
	}()
	wafSysLog := &model.WafSysLog{
		Id:         uuid.NewV4().String(),
		UserCode:   global.GWAF_USER_CODE,
		TenantId:   global.GWAF_TENANT_ID,
		OpType:     "信息",
		OpContent:  "WAF关闭",
		CreateTime: time.Now(),
	}
	global.GQEQUE_LOG_DB.PushBack(wafSysLog)
	waf.EngineCurrentStatus = 0
	for _, v := range waf.ServerOnline {
		if v.Svr != nil {
			v.Svr.Close()
		}
	}

	//重置信息

	waf.HostTarget = map[string]*wafenginmodel.HostSafe{}
	waf.HostCode = map[string]string{}
	waf.ServerOnline = map[int]innerbean.ServerRunTime{}
	waf.AllCertificate = map[int]map[string]*tls.Certificate{}

}
