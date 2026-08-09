package apiconnect

import (
	"purser/internal/service"

	settingsv1 "purser/gen/go/purser/settings/v1"
)

func settingSourceToProto(s service.SettingSource) settingsv1.SettingSource {
	switch s {
	case service.SettingSourceDefault:
		return settingsv1.SettingSource_SETTING_SOURCE_DEFAULT
	case service.SettingSourceEnv:
		return settingsv1.SettingSource_SETTING_SOURCE_ENV
	case service.SettingSourceYAML:
		return settingsv1.SettingSource_SETTING_SOURCE_YAML
	case service.SettingSourceDB:
		return settingsv1.SettingSource_SETTING_SOURCE_DB
	default:
		return settingsv1.SettingSource_SETTING_SOURCE_UNSPECIFIED
	}
}

func settingLockReasonToProto(r service.SettingLockReason) settingsv1.SettingLockReason {
	switch r {
	case service.SettingLockReasonBootstrap:
		return settingsv1.SettingLockReason_SETTING_LOCK_REASON_BOOTSTRAP
	case service.SettingLockReasonOperator:
		return settingsv1.SettingLockReason_SETTING_LOCK_REASON_OPERATOR
	default:
		return settingsv1.SettingLockReason_SETTING_LOCK_REASON_UNSPECIFIED
	}
}

func settingToProto(v service.SettingView) *settingsv1.Setting {
	return &settingsv1.Setting{
		Key:        v.Key,
		Value:      v.Value,
		Source:     settingSourceToProto(v.Source),
		Locked:     v.Locked,
		LockReason: settingLockReasonToProto(v.LockReason),
		Secret:     v.Secret,
	}
}

func settingsToProto(views []service.SettingView) []*settingsv1.Setting {
	pb := make([]*settingsv1.Setting, 0, len(views))
	for _, v := range views {
		pb = append(pb, settingToProto(v))
	}
	return pb
}
