package api

import (
	"SamWaf/global"
	"SamWaf/model"
	"SamWaf/model/common/response"
	"SamWaf/model/request"
	"SamWaf/utils"
	"errors"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
	"strings"
)

type WafRuleAPi struct {
}

func (w *WafRuleAPi) AddApi(c *gin.Context) {
	ruleHelper := &utils.RuleHelper{}
	var req request.WafRuleAddReq
	err := c.ShouldBind(&req)
	if err == nil {
		var ruleTool = model.RuleTool{}
		ruleInfo, err := ruleTool.LoadRule(req.RuleJson)
		if err != nil {
			response.FailWithMessage("规则解析错误", c)
			return
		}
		err = wafRuleService.CheckIsExistApi(ruleInfo.RuleBase.RuleName, ruleInfo.RuleBase.RuleDomainCode)
		if err != nil {
			response.FailWithMessage("当前规则名称已存在", c)
		}
		chsName := ruleInfo.RuleBase.RuleName
		var ruleCode = uuid.NewV4().String()
		ruleInfo.RuleBase.RuleName = strings.Replace(ruleCode, "-", "", -1)

		var ruleContent = ruleTool.GenRuleInfo(ruleInfo, chsName)
		if req.IsManualRule == 1 {
			ruleContent = ruleInfo.RuleContent
			//检查规则是否合法
			err = ruleHelper.CheckRuleAvailable(ruleContent)
			if err != nil {
				response.FailWithMessage("规则校验失败", c)
				return
			}
		}

		err = wafRuleService.AddApi(req, ruleCode, chsName, ruleInfo.RuleBase.RuleDomainCode, ruleContent)
		if err == nil {
			w.NotifyWaf(ruleInfo.RuleBase.RuleDomainCode)
			response.OkWithMessage("添加成功", c)
		} else {

			response.FailWithMessage("添加失败", c)
		}
	} else {
		response.FailWithMessage("解析失败", c)
	}
}
func (w *WafRuleAPi) GetDetailApi(c *gin.Context) {
	var req request.WafRuleDetailReq
	err := c.ShouldBind(&req)
	if err == nil {
		wafHost := wafRuleService.GetDetailApi(req)
		response.OkWithDetailed(wafHost, "获取成功", c)
	} else {
		response.FailWithMessage("解析失败", c)
	}
}
func (w *WafRuleAPi) GetListApi(c *gin.Context) {
	var req request.WafRuleSearchReq
	err := c.ShouldBind(&req)
	if err == nil {
		wafRules, total, _ := wafRuleService.GetListApi(req)
		response.OkWithDetailed(response.PageResult{
			List:      wafRules,
			Total:     total,
			PageIndex: req.PageIndex,
			PageSize:  req.PageSize,
		}, "获取成功", c)
	} else {
		response.FailWithMessage("解析失败", c)
	}
}

func (w *WafRuleAPi) GetListByHostCodeApi(c *gin.Context) {
	var req request.WafRuleSearchReq
	err := c.ShouldBind(&req)
	if err == nil {
		wafRules, total, _ := wafRuleService.GetListByHostCodeApi(req)
		response.OkWithDetailed(response.PageResult{
			List:      wafRules,
			Total:     total,
			PageIndex: req.PageIndex,
			PageSize:  req.PageSize,
		}, "获取成功", c)
	} else {
		response.FailWithMessage("解析失败", c)
	}
}
func (w *WafRuleAPi) DelRuleApi(c *gin.Context) {
	var req request.WafRuleDelReq
	err := c.ShouldBind(&req)
	if err == nil {
		wafRule := wafRuleService.GetDetailByCodeApi(req.CODE)
		err = wafRuleService.DelRuleApi(req)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			response.FailWithMessage("请检测参数", c)
		} else if err != nil {
			response.FailWithMessage("发生错误", c)
		} else {
			w.NotifyWaf(wafRule.HostCode)
			response.OkWithMessage("删除成功", c)
		}

	} else {
		response.FailWithMessage("解析失败", c)
	}
}

func (w *WafRuleAPi) ModifyRuleApi(c *gin.Context) {
	ruleHelper := &utils.RuleHelper{}
	var req request.WafRuleEditReq
	err := c.ShouldBind(&req)
	if err == nil {
		var ruleTool = model.RuleTool{}
		ruleInfo, err := ruleTool.LoadRule(req.RuleJson)
		if err != nil {
			response.FailWithMessage("解析错误", c)
			return
		}
		rule := wafRuleService.GetDetailByCodeApi(req.CODE)
		var ruleName = ruleInfo.RuleBase.RuleName //中文名
		ruleInfo.RuleBase.RuleName = strings.Replace(rule.RuleCode, "-", "", -1)
		var ruleContent = ruleTool.GenRuleInfo(ruleInfo, ruleName)
		if req.IsManualRule == 1 {
			ruleContent = ruleInfo.RuleContent
			//检查规则是否合法
			err = ruleHelper.CheckRuleAvailable(ruleContent)
			if err != nil {
				response.FailWithMessage("规则校验失败", c)
				return
			}
		}

		err = wafRuleService.ModifyApi(req, ruleName, ruleInfo.RuleBase.RuleDomainCode, ruleContent)
		if err != nil {
			response.FailWithMessage("编辑发生错误", c)
		} else {
			w.NotifyWaf(ruleInfo.RuleBase.RuleDomainCode)
			response.OkWithMessage("编辑成功", c)
		}

	} else {
		response.FailWithMessage("解析失败", c)
	}
}

/*
*
通知到waf引擎实时生效
*/
func (w *WafRuleAPi) NotifyWaf(host_code string) {
	var ruleconfig []model.Rules
	global.GWAF_LOCAL_DB.Debug().Where("host_code = ? ", host_code).Find(&ruleconfig)
	global.GWAF_CHAN_RULE <- ruleconfig
}
