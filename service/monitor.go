package service

import (
	"errors"
	"h-ui/dao"
	"h-ui/model/constant"
	"h-ui/model/vo"
	"h-ui/util"
)

const dashboardTrafficLimit = 10

func MonitorSystem() (vo.SystemMonitorVo, error) {
	cpuPercent, err := util.GetCpuPercent()
	if err != nil {
		return vo.SystemMonitorVo{}, errors.New("cpu query failed")
	}
	memPercent, err := util.GetMemPercent()
	if err != nil {
		return vo.SystemMonitorVo{}, errors.New("mem query failed")
	}
	diskPercent, err := util.GetDiskPercent()
	if err != nil {
		return vo.SystemMonitorVo{}, errors.New("disk query failed")
	}
	return vo.SystemMonitorVo{
		HUIVersion:  constant.Version,
		CpuPercent:  cpuPercent,
		MemPercent:  memPercent,
		DiskPercent: diskPercent,
	}, nil
}

func MonitorHysteria2() (vo.Hysteria2MonitorVo, error) {
	var hysteria2MonitorVo vo.Hysteria2MonitorVo
	hysteria2MonitorVo.Version = "-"
	version, err := util.InstalledHysteria2Version()
	if err == nil {
		hysteria2MonitorVo.Version = version
	}

	hysteria2MonitorVo.Running = Hysteria2IsRunning()
	if !hysteria2MonitorVo.Running {
		return hysteria2MonitorVo, nil
	}

	onlineUsers, err := Hysteria2Online()
	if err != nil {
		return hysteria2MonitorVo, err
	}

	if len(onlineUsers) > 0 {
		hysteria2MonitorVo.UserTotal = int64(len(onlineUsers))
		var deviceTotal int64 = 0
		for _, value := range onlineUsers {
			deviceTotal += value
		}
		hysteria2MonitorVo.DeviceTotal = deviceTotal
	}

	return hysteria2MonitorVo, nil
}

func MonitorTraffic() (vo.TrafficMonitorVo, error) {
	upload, download, rows, err := dao.AccountTrafficSummary(dashboardTrafficLimit)
	if err != nil {
		return vo.TrafficMonitorVo{}, err
	}
	result := vo.TrafficMonitorVo{
		Upload:   upload,
		Download: download,
		Total:    upload + download,
		TopUsers: make([]vo.AccountTrafficVo, 0, len(rows)),
	}
	for _, row := range rows {
		result.TopUsers = append(result.TopUsers, vo.AccountTrafficVo{
			Username: row.Username,
			Upload:   row.Upload,
			Download: row.Download,
			Total:    row.Upload + row.Download,
		})
	}
	return result, nil
}
