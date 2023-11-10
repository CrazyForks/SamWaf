package global

import (
	"SamWaf/cache"
	"SamWaf/model"
	"SamWaf/model/spec"
	"SamWaf/wafsnowflake"
	"github.com/bytedance/godlp/dlpheader"
	Dequelib "github.com/edwingeng/deque"
	"gorm.io/gorm"
	"strconv"
	"time"
)

const (
	GWAF_NAME   = "SamWaf"
	Version_num = 1
)

var (

	/*本机信息**/
	GWAF_RUNTIME_IP          string = "127.0.0.1" //本机当前外网IP
	GWAF_RUNTIME_AREA        string = ""          //本机当前所在区域
	GWAF_RUNTIME_SERVER_TYPE bool   = false       //当前是是否以服务形式启动

	GWAF_RUNTIME_NEW_VERSION      string = "" //最新版本号
	GWAF_RUNTIME_NEW_VERSION_DESC string = "" //最新版本描述

	GWAF_GLOBAL_HOST_NAME string = "全局网站:0" //全局网站

	GWAF_LOCAL_DB             *gorm.DB             //通用本地数据库，尊重用户隐私
	GWAF_LOCAL_LOG_DB         *gorm.DB             //通用本地数据库存日志数据，尊重用户隐私
	GWAF_LOCAL_STATS_DB       *gorm.DB             //通用本地数据库存放统计数据，尊重用户隐私
	GWAF_REMOTE_DB            *gorm.DB             //仅当用户使用云数据库
	GWAF_LOCAL_SERVER_PORT    int       = 26666    // 本地local端口
	GWAF_USER_CODE            string               // 当前识别号
	GWAF_CUSTOM_SERVER_NAME   string               // 当前服务器自定义名称
	GWAF_TENANT_ID            string               // 当前租户ID
	GWAF_RELEASE              string    = "false"  // 当前是否为发行版
	GWAF_RELEASE_VERSION_NAME string    = "1.0"    // 发行版的版本号名称
	GWAF_RELEASE_VERSION      string    = "v1.0.0" // 发行版的版本号
	GWAF_LAST_UPDATE_TIME     time.Time            // 上次时间

	GWAF_DLP dlpheader.EngineAPI // 脱敏引擎

	/**链聚合**/
	GWAF_CHAN_HOST   = make(chan model.Hosts, 10)         //主机链
	GWAF_CHAN_ENGINE = make(chan int, 10)                 //引擎链
	GWAF_CHAN_MSG    = make(chan spec.ChanCommonHost, 10) //全局通讯包
	GWAF_CHAN_UPDATE = make(chan int, 10)                 //升级后处理链

	/*****CACHE相关*********/
	GCACHE_WAFCACHE      *cache.WafCache      //cache
	GCACHE_WECHAT_ACCESS string          = "" //微信访问密钥
	GCACHE_IP_CBUFF      []byte               // IP相关缓存

	GDATA_DELETE_INTERVAL = 100 // 删除100天前的数据

	/****队列相关*****/
	GQEQUE_DB              Dequelib.Deque //正常DB队列
	GQEQUE_LOG_DB          Dequelib.Deque //日志DB队列
	GQEQUE_STATS_DB        Dequelib.Deque //统计DB队列
	GQEQUE_STATS_UPDATE_DB Dequelib.Deque //统计更新DB队列
	GQEQUE_MESSAGE_DB      Dequelib.Deque //发送消息队列

	/******WebSocket*********/
	GWebSocket *model.WebSocketOnline

	/******记录参数配置****************/
	GCONFIG_RECORD_MAX_BODY_LENGTH     int64 = 1024 * 2 //限制记录最大请求的body长度 record_max_req_body_length
	GCONFIG_RECORD_MAX_RES_BODY_LENGTH int64 = 1024 * 4 //限制记录最大响应的body长度 record_max_rep_body_length
	GCONFIG_RECORD_RESP                int64 = 0        // 是否记录响应记录 record_resp

	//升级相关
	GUPDATE_VERSION_URL string = "https://update.samwaf.com/" //

	GWAF_SNOWFLAKE_GEN *wafsnowflake.Snowflake //雪花算法

	//任务开关信息
	GWAF_SWITCH_TASK_COUNTER bool
)

func GetCurrentVersionInt() int {
	version, _ := strconv.Atoi(GWAF_RELEASE_VERSION)
	return version
}
