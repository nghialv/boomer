package boomer

import (
	"fmt"
	"reflect"

	"github.com/asaskevich/EventBus"
)

var events = EventBus.New()

func init() {
	events.Subscribe("request_success", requestSuccessHandler)
	events.Subscribe("request_failure", requestFailureHandler)
}

// According to locust, responseTime should be int64, in milliseconds.
// But previous version of boomer required responseTime to be float64, so sad.
func convertResponseTime(origin interface{}) int64 {
	if v, ok := origin.(float64); ok {
		return int64(v)
	}
	if v, ok := origin.(int64); ok {
		return v
	}
	panic(fmt.Sprintf("responseTime should be float64 or int64, not %s", reflect.TypeOf(origin)))
}

func requestSuccessHandler(requestType string, name string, responseTime interface{}, responseLength int64) {
	requestSuccessCh <- &requestSuccess{
		requestType:    requestType,
		name:           name,
		responseTime:   convertResponseTime(responseTime),
		responseLength: responseLength,
	}
}

func requestFailureHandler(requestType string, name string, responseTime interface{}, exception string) {
	requestFailureCh <- &requestFailure{
		requestType:  requestType,
		name:         name,
		responseTime: convertResponseTime(responseTime),
		error:        exception,
	}
}
