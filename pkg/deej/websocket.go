package deej

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/MattRCole/deej/pkg/deej/util"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type WebSocket struct {
	// hostName string
	// hostPort int
	deej   *Deej
	logger *zap.SugaredLogger

	connected bool
	conn      *websocket.Conn
	connUrl   string

	stopChannel                chan bool
	lastKnownNumSliders        int
	currentSliderPercentValues []float32
	sliderMoveConsumers        []chan SliderMoveEvent
}

// type SliderMoveEvent struct {
// 	SliderID     int
// 	PercentValue float32
// }
var expectedLinePatternWS = regexp.MustCompile(`^\d{1,4}(\|\d{1,4})*$`)

func NewWebSocket(deej *Deej, logger *zap.SugaredLogger) (*WebSocket, error) {
	// var addr = flag.String("addr", "deej.local:80", "http address of deej")
	// u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	// logger = logger.Named("websocket")
	// c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	ws := &WebSocket{
		deej:   deej,
		logger: logger,
		// connUrl: u.String(),
		conn:                nil,
		connected:           false,
		sliderMoveConsumers: []chan SliderMoveEvent{},
		stopChannel:         make(chan bool),
	}

	logger.Debug("Created websocket instance")

	// ws.setupOnConfigReload()

	return ws, nil
}

func (ws *WebSocket) Start() error {

	// don't allow multiple concurrent connections
	if ws.connected {
		ws.logger.Warn("Already connected, can't start another without closing first")
		return errors.New("websocket: connection already active")
	}

	u := url.URL{Scheme: "ws", Host: "deej.local:80", Path: "/ws"}
	ws.connUrl = u.String()

	ws.logger.Debugw("Attempting ws connection",
		"connUrl", ws.connUrl)

	var err error
	var _ *http.Response
	ws.conn, _, err = websocket.DefaultDialer.Dial(ws.connUrl, nil)

	if err != nil {

		// might need a user notification here, TBD
		ws.logger.Warnw("Failed to open websocket connection", "error", err)
		return fmt.Errorf("open websocket connection: %w", err)
	}

	namedLogger := ws.logger.Named(strings.ToLower(ws.connUrl))

	namedLogger.Infow("Connected", "conn", ws.conn)
	ws.connected = true

	// read lines or await a stop
	go func() {
		lineChannel := ws.readLine(namedLogger, ws.conn)

		for {
			select {
			case <-ws.stopChannel:
				ws.close(namedLogger)
			case line := <-lineChannel:
				ws.handleLine(namedLogger, line)
			}
		}
	}()

	return nil
}

func (ws *WebSocket) Stop() {
	if ws.connected {
		ws.logger.Debug("Shutting down websocket connection")
		ws.stopChannel <- true
	} else {
		ws.logger.Debug("Not currently connected, nothing to stop")
	}
}

func (ws *WebSocket) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
	ch := make(chan SliderMoveEvent)
	ws.sliderMoveConsumers = append(ws.sliderMoveConsumers, ch)

	return ch
}

// func (ws *WebSocket) setupOnConfigReload() {
// 	configReloadedChannel := ws.deej.config.SubscribeToChanges()
//
// 	const stopDelay = 50 * time.Millisecond
//
// 	go func() {
// 		for {
// 			select {
// 			case <-configReloadedChannel:
//
// 				// make any config reload unset our slider number to ensure process volumes are being re-set
// 				// (the next read line will emit SliderMoveEvent instances for all sliders)\
// 				// this needs to happen after a small delay, because the session map will also re-acquire sessions
// 				// whenever the config file is reloaded, and we don't want it to receive these move events while the map
// 				// is still cleared. this is kind of ugly, but shouldn't cause any issues
// 				go func() {
// 					<-time.After(stopDelay)
// 					ws.lastKnownNumSliders = 0
// 				}()
//
// 				newAddr := flag.String()
// 				maybeNewUrl =
// 				// if connection params have changed, attempt to stop and start the connection
// 				if ws.deej.config.ConnectionInfo.COMPort != ws.connOptions.PortName ||
// 					uint(ws.deej.config.ConnectionInfo.BaudRate) != sio.connOptions.BaudRate {
//
// 					sio.logger.Info("Detected change in connection parameters, attempting to renew connection")
// 					sio.Stop()
//
// 					// let the connection close
// 					<-time.After(stopDelay)
//
// 					if err := sio.Start(); err != nil {
// 						sio.logger.Warnw("Failed to renew connection after parameter change", "error", err)
// 					} else {
// 						sio.logger.Debug("Renewed connection successfully")
// 					}
// 				}
// 			}
// 		}
// 	}()
// }

func (ws *WebSocket) close(logger *zap.SugaredLogger) {
	logger.Debug("Closing Time!!!!")

	err := ws.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	if err != nil {
		logger.Warnw("Failed to close websocket connection", "error", err)
	} else {
		logger.Debug("websocket connection closed")
	}

	ws.conn = nil
	ws.connected = false
}

func (ws *WebSocket) readLine(logger *zap.SugaredLogger, conn *websocket.Conn) chan string {
	ch := make(chan string)

	go func() {
		for {
			mt, message, err := ws.conn.ReadMessage()
			if err != nil {
				logger.Errorf("recieved: %s, message type: %d", message, mt)
				ws.Stop()
				return
				// } else if mt == 1 {
				// 	logger.Warn("Got a binary message, cannot read!")
			} else {
				logger.Debug("Got a message!")
				ch <- string(message[:])
			}
		}
	}()
	return ch
}

func (ws *WebSocket) handleLine(logger *zap.SugaredLogger, line string) {
	logger.Debugf("Handling line: %s", line)

	// this function receives an unsanitized line which is guaranteed to end with LF,
	// but most lines will end with CRLF. it may also have garbage instead of
	// deej-formatted values, so we must check for that! just ignore bad ones
	if !expectedLinePatternWS.MatchString(line) {
		logger.Debugf("Unexpected line: %s", line)
		return
	}

	// trim the suffix
	// line = strings.TrimSuffix(line, "\r\n")

	// split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
	splitLine := strings.Split(line, "|")
	numSliders := len(splitLine)

	// update our slider count, if needed - this will send slider move events for all
	if numSliders != ws.lastKnownNumSliders {
		logger.Infow("Detected sliders", "amount", numSliders)
		ws.lastKnownNumSliders = numSliders
		ws.currentSliderPercentValues = make([]float32, numSliders)

		// reset everything to be an impossible value to force the slider move event later
		for idx := range ws.currentSliderPercentValues {
			ws.currentSliderPercentValues[idx] = -1.0
		}
	}

	var maxValue int = ws.deej.config.SliderMaximumValue
	var minValue int = ws.deej.config.SliderMinimumValue
	// for each slider:
	moveEvents := []SliderMoveEvent{}
	for sliderIdx, stringValue := range splitLine {

		// convert string values to integers ("1023" -> 1023)
		number, _ := strconv.Atoi(stringValue)

		// turns out the first line could come out dirty sometimes (i.e. "4558|925|41|643|220")
		// so let's check the first number for correctness just in case
		if sliderIdx == 0 && number > maxValue {
			ws.logger.Debugw("Got malformed line from websocket, ignoring", "line", line)
			return
		}

		// map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
		num := float32(math.Min(math.Max(float64(number-minValue), 0.0), float64(maxValue-minValue)))
		dirtyFloat := num / float32(maxValue-minValue)

		// normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
		normalizedScalar := util.NormalizeScalar(dirtyFloat)

		// if sliders are inverted, take the complement of 1.0
		if ws.deej.config.InvertSliders {
			normalizedScalar = 1 - normalizedScalar
		}

		// check if it changes the desired state (could just be a jumpy raw slider value)
		if util.SignificantlyDifferent(ws.currentSliderPercentValues[sliderIdx], normalizedScalar, ws.deej.config.NoiseReductionLevel) {

			// if it does, update the saved value and create a move event
			ws.currentSliderPercentValues[sliderIdx] = normalizedScalar

			moveEvents = append(moveEvents, SliderMoveEvent{
				SliderID:     sliderIdx,
				PercentValue: normalizedScalar,
			})

			if ws.deej.Verbose() {
				logger.Debugw("Slider moved", "event", moveEvents[len(moveEvents)-1])
			}
		}
	}

	// deliver move events if there are any, towards all potential consumers
	if len(moveEvents) > 0 {
		for _, consumer := range ws.sliderMoveConsumers {
			for _, moveEvent := range moveEvents {
				consumer <- moveEvent
			}
		}
	}
}
