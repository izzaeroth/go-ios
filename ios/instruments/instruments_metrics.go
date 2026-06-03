package instruments

import (
	"github.com/danielpaulus/go-ios/ios"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/golog"
)

type metricsDispatcher struct {
	messageChannel chan dtx.Message
	closeChannel   chan struct{}
}

func (dispatcher metricsDispatcher) Dispatch(msg dtx.Message) {
	golog.Info("metrics message", "message", msg)
}

func GetMetrics(device ios.DeviceEntry) (func() (map[string]interface{}, error), func() error, error) {
	conn, err := connectInstruments(device)
	if err != nil {
		return nil, nil, err
	}
	dispatcher := metricsDispatcher{messageChannel: make(chan dtx.Message), closeChannel: make(chan struct{})}
	conn.AddDefaultChannelReceiver(dispatcher)
	channel := conn.RequestChannelIdentifier(mobileNotificationsChannel, channelDispatcher{})
	resp, err := channel.MethodCall("setApplicationStateNotificationsEnabled:", true)
	if err != nil {
		golog.Error("setApplicationStateNotificationsEnabled failed", "response", resp, "payload", resp.Payload[0])
		return nil, nil, err
	}
	golog.Debug("appstatenotifications enabled successfully", "response", resp)
	resp, err = channel.MethodCall("setMemoryNotificationsEnabled:", true)
	if err != nil {
		golog.Error("setMemoryNotificationsEnabled failed", "response", resp, "payload", resp.Payload[0])
		return nil, nil, err
	}
	golog.Debug("memory notifications enabled", "response", resp)

	return nil, nil, nil
}
