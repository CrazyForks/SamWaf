package globalobj

import (
	"SamWaf/wafenginecore"
	"SamWaf/waftask"
	"SamWaf/waftunnelengine"
)

var (
	/***
	本地对象映射关系
	*/
	GWAF_RUNTIME_OBJ_WAF_ENGINE        *wafenginecore.WafEngine         //当前引擎对象
	GWAF_RUNTIME_OBJ_TUNNEL_ENGINE     *waftunnelengine.WafTunnelEngine //当前tunnel引擎对象
	GWAF_RUNTIME_OBJ_WAF_TaskRegistry  *waftask.TaskRegistry            // 任务执行器
	GWAF_RUNTIME_OBJ_WAF_TaskScheduler *waftask.TaskScheduler           // 任务计划
)
