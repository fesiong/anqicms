package provider

import (
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetOrderServices(ops func(tx *gorm.DB) *gorm.DB, currentPage, pageSize int, offsets ...int) ([]*model.OrderService, int64) {
	var services []*model.OrderService
	var offset int
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	if len(offsets) > 0 {
		offset = offsets[0]
	}

	var total int64

	tx := w.DB.Model(&model.OrderService{})
	if ops != nil {
		tx = ops(tx)
	}
	tx.Count(&total)
	tx.Order("sort desc,id desc").Offset(offset).Limit(pageSize).Find(&services)

	return services, total
}

func (w *Website) GetOrderService(id int64) (*model.OrderService, error) {
	var service model.OrderService
	err := w.DB.First(&service, id).Error
	if err != nil {
		return nil, err
	}
	return &service, nil
}

func (w *Website) SaveOrderService(req *request.OrderServiceRequest) error {
	var service model.OrderService
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).Take(&service).Error
		if err != nil {
			return err
		}
	}
	service.ServiceCode = req.ServiceCode
	service.ServiceName = req.ServiceName
	service.ServiceType = req.ServiceType
	service.Description = req.Description
	service.Enabled = req.Enabled
	service.Sort = req.Sort
	service.Fee = req.Fee
	service.FeeType = req.FeeType
	service.MaxFee = req.MaxFee
	service.Extra = req.Extra

	return w.DB.Save(&service).Error
}

func (w *Website) DeleteOrderService(id int64) error {
	var exist model.OrderService
	err := w.DB.Where("`id` = ?", id).Take(&exist).Error
	if err != nil {
		return err
	}

	return w.DB.Where("`id` = ?", id).Delete(&model.OrderService{}).Error
}

func (w *Website) GetOrderServicesByOrderId(orderId string) ([]*model.OrderServiceAddon, error) {
	var services []*model.OrderServiceAddon
	err := w.DB.Where("`order_id` = ?", orderId).Find(&services).Error
	if err != nil {
		return nil, err
	}
	if len(services) > 0 {
		var serviceIds []int64
		for _, service := range services {
			serviceIds = append(serviceIds, service.ServiceId)
		}
		var orderServices []*model.OrderService
		w.DB.Where("`id` IN (?)", serviceIds).Find(&orderServices)
		for _, service := range services {
			for _, orderService := range orderServices {
				if service.ServiceId == orderService.Id {
					service.OrderService = orderService
					break
				}
			}
		}
	}

	return services, nil
}
