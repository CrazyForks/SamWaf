package waf_service

import (
	"SamWaf/global"
	"SamWaf/model"
	"SamWaf/model/request"
	response2 "SamWaf/model/response"
	"SamWaf/utils"
	"fmt"
	"github.com/shirou/gopsutil/host"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

type WafStatService struct{}

var WafStatServiceApp = new(WafStatService)

func (receiver *WafStatService) StatHomeSumDayApi() (response2.WafStat, error) {
	currentDay, _ := strconv.Atoi(time.Now().Format("20060102"))
	yesterdayDay, _ := strconv.Atoi(time.Now().AddDate(0, 0, -1).Format("20060102"))

	var AttackCountOfToday int64
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsDay{}).Where("day = ? and type = ? ",
		currentDay, "阻止").Select("sum(count) as vcnt").Row().Scan(&AttackCountOfToday)

	var VisitCountOfToday int64
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsDay{}).Where("day = ? ",
		currentDay).Select("sum(count) as vcnt").Row().Scan(&VisitCountOfToday)

	var AttackCountOfYesterday int64
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsDay{}).Where("day = ? and type = ? ",
		yesterdayDay, "阻止").Select("sum(count) as vcnt").Row().Scan(&AttackCountOfYesterday)

	var VisitCountOfYesterday int64
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsDay{}).Where("day = ? ",
		yesterdayDay).Select("sum(count) as vcnt").Row().Scan(&VisitCountOfYesterday)

	var NormalIpCountOfToday int64
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsIPDay{}).Where("day = ? and type = ? ",
		currentDay, "放行").Group("ip").Count(&NormalIpCountOfToday)

	var IllegalIpCountOfToday int64
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsIPDay{}).Where("day = ? and type = ? ",
		currentDay, "阻止").Group("ip").Count(&IllegalIpCountOfToday)
	return response2.WafStat{
			AttackCountOfToday:          AttackCountOfToday,
			VisitCountOfToday:           VisitCountOfToday,
			AttackCountOfYesterday:      AttackCountOfYesterday,
			VisitCountOfYesterday:       VisitCountOfYesterday,
			AttackCountOfLastWeekToday:  0,
			VisitCountOfLastWeekToday:   0,
			NormalIpCountOfToday:        NormalIpCountOfToday,
			IllegalIpCountOfToday:       IllegalIpCountOfToday,
			NormalCountryCountOfToday:   0,
			IllegalCountryCountOfToday:  0,
			NormalProvinceCountOfToday:  0,
			IllegalProvinceCountOfToday: 0,
			NormalCityCountOfToday:      0,
			IllegalCityCountOfToday:     0,
			CurrentQps:                  atomic.LoadUint64(&global.GWAF_RUNTIME_QPS),
		},
		nil
}

func (receiver *WafStatService) StatHomeSumDayRangeApi(req request.WafStatsDayRangeReq) (response2.WafStatRange, error) {
	var rangeAttackMap = map[int]int64{}
	var rangeNormalMap = map[int]int64{}
	var rangeInt = (int)(utils.Str2Time(req.EndDay).Sub(utils.Str2Time(req.StartDay)).Hours() / 24)

	for i := 0; i < rangeInt; i++ {
		rangeAttackMap[utils.TimeToDayInt(utils.Str2Time(req.StartDay).AddDate(0, 0, i))] = 0
		rangeNormalMap[utils.TimeToDayInt(utils.Str2Time(req.StartDay).AddDate(0, 0, i))] = 0
	}

	var AttackCountOfRange []model.StatsDayCount
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsDay{}).Where("day between ? and ? and type = ? ",
		req.StartDay, req.EndDay, "阻止").Select("day,sum(count) as count").Group("day").Scan(&AttackCountOfRange)
	var NormalCountOfRange []model.StatsDayCount
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsDay{}).Where("day between ? and ? and type = ? ",
		req.StartDay, req.EndDay, "放行").Select("day,sum(count) as count").Group("day").Scan(&NormalCountOfRange)

	for i := 0; i < len(AttackCountOfRange); i++ {
		bean := AttackCountOfRange[i]
		_, ok := rangeAttackMap[bean.Day]
		if ok {
			rangeAttackMap[bean.Day] = bean.Count
		}
	}
	for i := 0; i < len(NormalCountOfRange); i++ {
		bean := NormalCountOfRange[i]
		_, ok := rangeNormalMap[bean.Day]
		if ok {
			rangeNormalMap[bean.Day] = bean.Count
		}
	}
	return response2.WafStatRange{
			AttackCountOfRange: rangeAttackMap,
			NormalCountOfRange: rangeNormalMap,
		},
		nil
}
func (receiver *WafStatService) StatHomeSumDayTopIPRangeApi(req request.WafStatsDayRangeReq) (response2.WafIPStats, error) {
	var AttackCountOfRange []model.StatsIPCount
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsIPDay{}).Where("day between ? and ? and type = ? ",
		req.StartDay, req.EndDay, "阻止").Select("ip,sum(count) as count").Group("ip").Order("sum(count) desc").Limit(10).Scan(&AttackCountOfRange)
	var NormalCountOfRange []model.StatsIPCount
	global.GWAF_LOCAL_STATS_DB.Model(&model.StatsIPDay{}).Where("day between ? and ? and type = ? ",
		req.StartDay, req.EndDay, "放行").Select("ip,sum(count) as count").Group("ip").Order("sum(count) desc").Limit(10).Scan(&NormalCountOfRange)

	return response2.WafIPStats{
			AttackIPOfRange: AttackCountOfRange,
			NormalIPOfRange: NormalCountOfRange,
		},
		nil
}

// 通过时间获取国家级别的 攻击数 访问数
func (receiver *WafStatService) StatAnalysisDayCountryRangeApi(req request.WafStatsAnalysisDayRangeCountryReq) []response2.WafAnalysisDayStats {
	var CountOfRange []response2.WafAnalysisDayStats

	global.GWAF_LOCAL_STATS_DB.Debug().Model(&model.StatsIPCityDay{}).Where("day between ? and ? and type = ? ",
		req.StartDay, req.EndDay, req.AttackType).Select(" country as Name ,sum(count) as Value").Group("country").Order("sum(count) desc").Scan(&CountOfRange)
	return CountOfRange
}

// 获取系统基本信息
func (receiver *WafStatService) StatHomeSysinfo() response2.WafHomeSysinfoStat {

	return response2.WafHomeSysinfoStat{IsDefaultAccount: WafAccountServiceApp.IsExistDefaultAccount(), IsEmptyHost: WafHostServiceApp.IsEmptyHost()}
}

// 获取运行系统基本信息
func (receiver *WafStatService) StatHomeRumtimeSysinfo() []response2.WafNameValue {
	/*c, _ := cpu.Info()
	cc, _ := cpu.Percent(time.Second, false) // 1秒
	d, _ := disk.Usage("/")
	n, _ := host.Info()
	nv, _ := net.IOCounters(true)
	physicalCnt, _ := cpu.Counts(false)
	logicalCnt, _ := cpu.Counts(true)
	result := ""

	if len(c) > 1 {
		for _, sub_cpu := range c {
			modelname := sub_cpu.ModelName
			cores := sub_cpu.Cores
			result = result + fmt.Sprintf("CPUs: %v   %v cores \n", modelname, cores)
		}
	} else if len(c) ==1{
		sub_cpu := c[0]
		modelname := sub_cpu.ModelName
		cores := sub_cpu.Cores
		result = result + fmt.Sprintf("CPU: %v   %v cores \n", modelname, cores)
	}
	result = result + fmt.Sprintf("physical count:%d logical count:%d\n", physicalCnt, logicalCnt)
	result = result + fmt.Sprintf("CPU Used: used %f%%\n", cc[0])
	result = result + fmt.Sprintf("HD: %v GB Free: %v GB Usage:%f%%\n", d.Total/1024/1024/1024, d.Free/1024/1024/1024, d.UsedPercent)
	result = result + fmt.Sprintf("OS: %v(%v) %v\n", n.Platform, n.PlatformFamily, n.PlatformVersion)
	result = result + fmt.Sprintf("Hostname: %v\n", n.Hostname)
	result = result + fmt.Sprintf("Network: %v bytes / %v bytes\n", nv[0].BytesRecv, nv[0].BytesSent)
	*/
	var data []response2.WafNameValue
	//data = append(data, response2.WafNameValue{Name: "系统运行环境基本信息", Value: result})
	data = append(data, response2.WafNameValue{Name: "最后处理log时间",
		Value: time.Unix(0,
			global.GWAF_MEASURE_PROCESS_DEQUEENGINE.ReadData()*int64(time.Millisecond)).Format("2006-01-02 15:04:05")})
	data = append(data, response2.WafNameValue{Name: "Goroutine数量", Value: fmt.Sprintf("%v", runtime.NumGoroutine())})
	data = append(data, response2.WafNameValue{Name: "系统类型", Value: fmt.Sprintf("%v", runtime.GOOS)})
	data = append(data, response2.WafNameValue{Name: "系统架构", Value: fmt.Sprintf("%v", runtime.GOARCH)})

	// 获取开机时间
	boottime, _ := host.BootTime()
	ntime := time.Now().Unix()
	btime := time.Unix(int64(boottime), 0).Unix()
	deltatime := ntime - btime
	// 将时间间隔转换为天、小时、分钟、秒
	seconds := int64(deltatime)
	minutes := seconds / 60
	seconds -= minutes * 60
	hours := minutes / 60
	minutes -= hours * 60
	days := hours / 24
	hours -= days * 24

	data = append(data, response2.WafNameValue{
		Name: "系统已运行时长", Value: fmt.Sprintf("%v 天 %v 时 %v 分 %v 秒", days, hours, minutes, seconds)})

	data = append(data, response2.WafNameValue{Name: "软件版本", Value: fmt.Sprintf("%v", global.GWAF_RELEASE_VERSION_NAME)})
	data = append(data, response2.WafNameValue{Name: "软件版本Code", Value: fmt.Sprintf("%v", global.GWAF_RELEASE_VERSION)})
	data = append(data, response2.WafNameValue{Name: "当前QPS", Value: fmt.Sprintf("%v", atomic.LoadUint64(&global.GWAF_RUNTIME_QPS))})

	data = append(data, response2.WafNameValue{Name: "当前队列数", Value: fmt.Sprintf("主数据：%v 日志数据：%v  统计数据：%v  消息队列：%v", global.GQEQUE_DB.Len(), global.GQEQUE_LOG_DB.Len(), global.GQEQUE_STATS_DB.Len(), global.GQEQUE_MESSAGE_DB.Len())})
	data = append(data, response2.WafNameValue{Name: "当前日志队列处理QPS", Value: fmt.Sprintf("%v", atomic.LoadUint64(&global.GWAF_RUNTIME_LOG_PROCESS))})

	return data
}
