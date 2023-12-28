package waf_service

import (
	"SamWaf/customtype"
	"SamWaf/global"
	"SamWaf/model"
	"SamWaf/model/baseorm"
	"SamWaf/model/request"
	"errors"
	uuid "github.com/satori/go.uuid"
	"time"
)

type WafWhiteUrlService struct{}

var WafWhiteUrlServiceApp = new(WafWhiteUrlService)

func (receiver *WafWhiteUrlService) AddApi(req request.WafWhiteUrlAddReq) error {
	var bean = &model.URLWhiteList{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.NewV4().String(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},
		HostCode:    req.HostCode,
		CompareType: req.CompareType,
		Url:         req.Url,
		Remarks:     req.Remarks,
	}
	global.GWAF_LOCAL_DB.Create(bean)
	return nil
}

func (receiver *WafWhiteUrlService) CheckIsExistApi(req request.WafWhiteUrlAddReq) error {
	return global.GWAF_LOCAL_DB.First(&model.URLWhiteList{}, "host_code = ? and url= ?", req.HostCode,
		req.Url).Error
}
func (receiver *WafWhiteUrlService) ModifyApi(req request.WafWhiteUrlEditReq) error {
	var ipWhite model.URLWhiteList
	global.GWAF_LOCAL_DB.Where("host_code = ? and url= ?", req.HostCode,
		req.Url).Find(&ipWhite)
	if ipWhite.Id != "" && ipWhite.Url != req.Url {
		return errors.New("当前网站和url已经存在")
	}
	ipWhiteMap := map[string]interface{}{
		"Host_Code":    req.HostCode,
		"Compare_Type": req.CompareType,
		"Url":          req.Url,
		"Remarks":      req.Remarks,
		"UPDATE_TIME":  customtype.JsonTime(time.Now()),
	}
	err := global.GWAF_LOCAL_DB.Model(model.URLWhiteList{}).Where("id = ?", req.Id).Updates(ipWhiteMap).Error

	return err
}
func (receiver *WafWhiteUrlService) GetDetailApi(req request.WafWhiteUrlDetailReq) model.URLWhiteList {
	var bean model.URLWhiteList
	global.GWAF_LOCAL_DB.Where("id=?", req.Id).Find(&bean)
	return bean
}
func (receiver *WafWhiteUrlService) GetDetailByIdApi(id string) model.URLWhiteList {
	var bean model.URLWhiteList
	global.GWAF_LOCAL_DB.Where("id=?", id).Find(&bean)
	return bean
}
func (receiver *WafWhiteUrlService) GetListApi(req request.WafWhiteUrlSearchReq) ([]model.URLWhiteList, int64, error) {
	var list []model.URLWhiteList
	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""
	if len(req.HostCode) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " host_code=? "
	}
	if len(req.Url) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " url =? "
	}
	//where字段赋值
	if len(req.HostCode) > 0 {
		whereValues = append(whereValues, req.HostCode)
	}
	if len(req.Url) > 0 {
		whereValues = append(whereValues, req.Url)
	}

	global.GWAF_LOCAL_DB.Model(&model.URLWhiteList{}).Where(whereField, whereValues...).Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Find(&list)
	global.GWAF_LOCAL_DB.Model(&model.URLWhiteList{}).Where(whereField, whereValues...).Count(&total)

	return list, total, nil
}
func (receiver *WafWhiteUrlService) DelApi(req request.WafWhiteUrlDelReq) error {
	var ipWhite model.URLWhiteList
	err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&ipWhite).Error
	if err != nil {
		return err
	}
	err = global.GWAF_LOCAL_DB.Where("id = ?", req.Id).Delete(model.URLWhiteList{}).Error
	return err
}
