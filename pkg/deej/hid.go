package deej

import (
	"errors"
	"math"

	"github.com/karalabe/hid"
	"github.com/mattrcole/deej/pkg/deej/util"
	"go.uber.org/zap"
)

type HIDeej struct {
	// hostName string
	// hostPort int
	deej   *Deej
	logger *zap.SugaredLogger

	connected bool
	device    *hid.Device

	stopChannel                chan bool
	lastKnownNumSliders        int
	currentSliderPercentValues []float32
	sliderMoveConsumers        []chan SliderMoveEvent
}

const (
	vendorId             = uint16(0x2341)
	productId            = uint16(0x0dee)
	volumeSliderReportId = 0x01
)

func NewHIDeej(deej *Deej, logger *zap.SugaredLogger) (*HIDeej, error) {
	hideej := &HIDeej{
		deej:   deej,
		logger: logger,
		// connUrl: u.String(),
		connected:           false,
		sliderMoveConsumers: []chan SliderMoveEvent{},
		stopChannel:         make(chan bool),
	}

	logger.Debug("Created HID instance")

	hideej.setupOnConfigReload()

	return hideej, nil
}

func (hideej *HIDeej) Start() error {
	// don't allow multiple concurrent connections
	if hideej.connected {
		hideej.logger.Warn("Already connected, can't start another without closing first")
		return errors.New("hid: connection already active")
	}

	logger := hideej.logger.Named("hid-setup")

	var err error
	if hid.Supported() == false {
		logger.Warn("HID NOT SUPPORTED!!!")
	}
	availableDevices := hid.Enumerate(vendorId, productId)
	if len(availableDevices) == 0 {
		allDevices := hid.Enumerate(0, 0)
		for _, info := range allDevices {
			logger.Debugf("Available device:")
			logger.Debugf("Vendor ID: 0x%04x, Product ID: 0x%04x", info.VendorID, info.ProductID)
			logger.Debugf("Usage: 0x%04x", info.Usage)
			logger.Debugf("Usage Page: 0x%04x", info.UsagePage)
			logger.Debugf("Path: %s", info.Path)
			logger.Debugf("Product: %s", info.Product)
			logger.Debugf("Manufacturer: %s", info.Manufacturer)
			logger.Debug("")
		}
		// enumFunc := func(info *hid.DeviceInfo) error {
		// 	return nil
		// }
		// logger.Errorf("Could not open device. err: %w", err)
		// hid.Enumerate(hid.VendorIDAny, hid.ProductIDAny, enumFunc)
		return errors.New("No HID device found with correct vendor id and product id")
	}
	deviceInfo := availableDevices[0]
	hideej.device, err = deviceInfo.Open()
	if err != nil {
		logger.Errorw("Could not open devie! err: %w", err)
		return err
	}
	// if err != nil {
	// 	enumFunc := func(info *hid.DeviceInfo) error {
	// 		logger.Debugf("Available device:")
	// 		logger.Debugf("Vendor ID: %x, Product ID: %x", info.VendorID, info.ProductID)
	// 		logger.Debugf("Product String: %s", info.ProductStr)
	// 		logger.Debugf("Usage Page: %x", info.UsagePage)
	// 		logger.Debugf("Path: %s", info.Path)
	// 		logger.Debug("")
	// 		return nil
	// 	}
	// 	logger.Errorf("Could not open device. err: %w", err)
	// 	hid.Enumerate(hid.VendorIDAny, hid.ProductIDAny, enumFunc)
	// 	return err
	// }
	// We'll just sit there and poll I guess
	// hideej.device.SetNonblock(true)

	namedLogger := hideej.logger.Named("hid")

	// devInfo, r := hideej.device.GetDeviceInfo()
	namedLogger.Infow("Connected", "device info", deviceInfo)
	hideej.connected = true

	// read lines or await a stop
	go func() {
		volumesChannel := hideej.readReport(namedLogger, hideej.device)

		for {
			select {
			case <-hideej.stopChannel:
				hideej.close(namedLogger)
			case volumes := <-volumesChannel:
				hideej.handleVolumeChange(namedLogger, volumes)
			}
		}
	}()

	return nil
}

func (hideej *HIDeej) Stop() {
	if hideej.connected {
		hideej.logger.Debug("Shutting down websocket connection")
		hideej.stopChannel <- true
	} else {
		hideej.logger.Debug("Not currently connected, nothing to stop")
	}
}

func (hideej *HIDeej) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
	ch := make(chan SliderMoveEvent)
	hideej.sliderMoveConsumers = append(hideej.sliderMoveConsumers, ch)

	return ch
}

func (hideej *HIDeej) setupOnConfigReload() {
	// configReloadedChannel := hideej.deej.config.SubscribeToChanges()

	// const stopDelay = 50 * time.Millisecond

	// go func() {
	// 	for range configReloadedChannel {
	// 		hideej.logger.Debug("Reloading config for websockets")

	// 		// make any config reload unset our slider number to ensure process volumes are being re-set
	// 		// (the next read line will emit SliderMoveEvent instances for all sliders)\
	// 		// this needs to happen after a small delay, because the session map will also re-acquire sessions
	// 		// whenever the config file is reloaded, and we don't want it to receive these move events while the map
	// 		// is still cleared. this is kind of ugly, but shouldn't cause any issues
	// 		go func() {
	// 			<-time.After(stopDelay)
	// 			hideej.lastKnownNumSliders = 0
	// 		}()

	// 		maybeNewHost := hideej.deej.config.Host
	// 		maybeNewPort := hideej.deej.config.Port
	// 		host := maybeNewHost + ":" + strconv.Itoa(maybeNewPort)

	// 		u := url.URL{Scheme: "hideej", Host: host, Path: "/hideej"}
	// 		// if connection host has changed, attempt to stop and start the connection
	// 		if u.String() != hideej.connUrl {

	// 			hideej.logger.Info("Detected change in host, attempting to renew connection")
	// 			hideej.Stop()

	// 			// let the connection close
	// 			<-time.After(stopDelay)

	// 			if err := hideej.Start(); err != nil {
	// 				hideej.logger.Warnw("Failed to renew connection after parameter change", "error", err)
	// 			} else {
	// 				hideej.logger.Debug("Renewed connection successfully")
	// 			}
	// 		}
	// 	}
	// }()
}

func (hideej *HIDeej) close(logger *zap.SugaredLogger) {
	// logger.Debug("Closing Time!!!!")

	err := hideej.device.Close()
	// err := hideej.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	if err != nil {
		logger.Warnw("Failed to close hid device", "error", err)
	} else {
		logger.Debug("hid device closed")
	}

	// hideej.conn = nil
	hideej.connected = false
}

func (hideej *HIDeej) readReport(logger *zap.SugaredLogger, device *hid.Device) chan []uint8 {
	ch := make(chan []uint8)

	go func() {
		for {
			var buffer [100]byte
			bytes_read, err := device.Read(buffer[:])
			if err != nil {
				logger.Errorf("Error reading from hid device %w", err)
				hideej.Stop()
				return
			} else {
				if buffer[0] != volumeSliderReportId {
					logger.Warnf("Got unknown report: %x", buffer[0])
				}
				logger.Debug("Got a message!")
				ch <- buffer[1:bytes_read]
			}
			// if err != nil && err != hid.ErrTimeout {
			// 	logger.Errorf("Error reading from hid device %w", err)
			// 	hideej.Stop()
			// 	return
			// } else if err == hid.ErrTimeout {
			// 	logger.Debug("Timed out, will wait for data")
			// 	time.Sleep(10 * time.Millisecond)
			// } else {
			// 	if buffer[0] != volumeSliderReportId {
			// 		logger.Warnf("Got unknown report: %x", buffer[0])
			// 	}
			// 	logger.Debug("Got a message!")
			// 	ch <- buffer[1:bytes_read]
			// }
		}
	}()
	return ch
}

func (hideej *HIDeej) handleVolumeChange(logger *zap.SugaredLogger, volumes []uint8) {
	logger.Debugf("Handling volume change: %w", volumes)

	numSliders := len(volumes)

	// update our slider count, if needed - this will send slider move events for all
	if numSliders != hideej.lastKnownNumSliders {
		logger.Infow("Detected sliders", "amount", numSliders)
		hideej.lastKnownNumSliders = numSliders
		hideej.currentSliderPercentValues = make([]float32, numSliders)

		// reset everything to be an impossible value to force the slider move event later
		for idx := range hideej.currentSliderPercentValues {
			hideej.currentSliderPercentValues[idx] = -1.0
		}
	}

	var maxValue int = hideej.deej.config.SliderMaximumValue
	var minValue int = hideej.deej.config.SliderMinimumValue
	// for each slider:
	moveEvents := []SliderMoveEvent{}
	for sliderIdx, number := range volumes {

		// map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
		num := float32(math.Min(math.Max(float64(int(number)-minValue), 0.0), float64(maxValue-minValue)))
		dirtyFloat := num / float32(maxValue-minValue)

		// normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
		normalizedScalar := util.NormalizeScalar(dirtyFloat)

		// if sliders are inverted, take the complement of 1.0
		if hideej.deej.config.InvertSliders {
			normalizedScalar = 1 - normalizedScalar
		}

		// check if it changes the desired state (could just be a jumpy raw slider value)
		if util.SignificantlyDifferent(hideej.currentSliderPercentValues[sliderIdx], normalizedScalar, hideej.deej.config.NoiseReductionLevel) {

			// if it does, update the saved value and create a move event
			hideej.currentSliderPercentValues[sliderIdx] = normalizedScalar

			moveEvents = append(moveEvents, SliderMoveEvent{
				SliderID:     sliderIdx,
				PercentValue: normalizedScalar,
			})

			if hideej.deej.Verbose() {
				logger.Debugw("Slider moved", "event", moveEvents[len(moveEvents)-1])
			}
		}
	}

	// deliver move events if there are any, towards all potential consumers
	if len(moveEvents) > 0 {
		for _, consumer := range hideej.sliderMoveConsumers {
			for _, moveEvent := range moveEvents {
				consumer <- moveEvent
			}
		}
	}
}
