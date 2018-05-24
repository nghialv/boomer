package boomer

import (
	"time"
)

type requestStats struct {
	entries   map[string]*statsEntry
	errors    map[string]*statsError
	total     *statsEntry
	startTime int64
}

type requestSuccess struct {
	requestType    string
	name           string
	responseTime   int64
	responseLength int64
}

type requestFailure struct {
	requestType  string
	name         string
	responseTime int64
	error        string
}

var (
	stats            = newRequestStats()
	requestSuccessCh = make(chan *requestSuccess, 100)
	requestFailureCh = make(chan *requestFailure, 100)
	clearStatsCh     = make(chan bool)
	messageToRunner  = make(chan map[string]interface{}, 10)
)

func init() {
	stats.entries = make(map[string]*statsEntry)
	stats.errors = make(map[string]*statsError)
	go func() {
		var ticker = time.NewTicker(slaveReportInterval)
		for {
			select {
			case m := <-requestSuccessCh:
				stats.logRequest(m.requestType, m.name, m.responseTime, m.responseLength)
			case n := <-requestFailureCh:
				stats.logError(n.requestType, n.name, n.error)
			case <-clearStatsCh:
				stats.clearAll()
			case <-ticker.C:
				data := collectReportData()
				// Send data to channel, no network IO in this goroutine
				messageToRunner <- data
			}
		}
	}()
}

func newRequestStats() *requestStats {
	stats := &requestStats{
		entries: make(map[string]*statsEntry),
		errors:  make(map[string]*statsError),
		total: &statsEntry{
			name:   "Total",
			method: "",
		},
	}
	stats.total.reset()
	return stats
}

func (s *requestStats) logRequest(method, name string, responseTime int64, contentLength int64) {
	s.total.log(responseTime, contentLength)
	s.get(name, method).log(responseTime, contentLength)
}

func (s *requestStats) logError(method, name, err string) {
	s.total.logError(err)
	s.get(name, method).logError(err)
	// Store error in errors map
	key := MD5(method, name, err)
	entry, ok := s.errors[key]
	if !ok {
		entry = &statsError{
			name:   name,
			method: method,
			error:  err,
		}
		s.errors[key] = entry
	}
	entry.occured()
}

func (s *requestStats) get(name string, method string) (entry *statsEntry) {
	entry, ok := s.entries[name+method]
	if !ok {
		newEntry := &statsEntry{
			name:          name,
			method:        method,
			numReqsPerSec: make(map[int64]int64),
			responseTimes: make(map[int64]int64),
		}
		newEntry.reset()
		s.entries[name+method] = newEntry
		return newEntry
	}
	return entry
}

func (s *requestStats) clearAll() {
	s.total = &statsEntry{
		name:   "Total",
		method: "",
	}
	s.total.reset()
	s.entries = make(map[string]*statsEntry)
	s.errors = make(map[string]*statsError)
	s.startTime = time.Now().Unix()
}

func (s *requestStats) serializeStats() []interface{} {
	entries := make([]interface{}, 0, len(s.entries))
	for _, v := range s.entries {
		if !(v.numRequests == 0 && v.numFailures == 0) {
			entries = append(entries, v.getStrippedReport())
		}
	}
	return entries
}

func (s *requestStats) serializeErrors() map[string]map[string]interface{} {
	errors := make(map[string]map[string]interface{})
	for k, v := range s.errors {
		errors[k] = v.toMap()
	}
	return errors
}

type statsEntry struct {
	name                 string
	method               string
	numRequests          int64
	numFailures          int64
	totalResponseTime    int64
	minResponseTime      int64
	maxResponseTime      int64
	numReqsPerSec        map[int64]int64
	responseTimes        map[int64]int64
	totalContentLength   int64
	startTime            int64
	lastRequestTimestamp int64
}

func (s *statsEntry) reset() {
	s.startTime = time.Now().Unix()
	s.numRequests = 0
	s.numFailures = 0
	s.totalResponseTime = 0
	s.responseTimes = make(map[int64]int64)
	s.minResponseTime = 0
	s.maxResponseTime = 0
	s.lastRequestTimestamp = time.Now().Unix()
	s.numReqsPerSec = make(map[int64]int64)
	s.totalContentLength = 0
}

func (s *statsEntry) log(responseTime int64, contentLength int64) {
	s.numRequests++
	s.logTimeOfRequest()
	s.logResponseTime(responseTime)
	s.totalContentLength += contentLength
}

func (s *statsEntry) logTimeOfRequest() {
	now := time.Now().Unix()
	_, ok := s.numReqsPerSec[now]
	if !ok {
		s.numReqsPerSec[now] = 1
	} else {
		s.numReqsPerSec[now]++
	}
	s.lastRequestTimestamp = now
}

func (s *statsEntry) logResponseTime(responseTime int64) {
	s.totalResponseTime += responseTime
	if s.minResponseTime == 0 {
		s.minResponseTime = responseTime
	}
	if responseTime < s.minResponseTime {
		s.minResponseTime = responseTime
	}
	if responseTime > s.maxResponseTime {
		s.maxResponseTime = responseTime
	}
	roundedResponseTime := int64(0)

	// to avoid to much data that has to be transferred to the master node when
	// running in distributed mode, we save the response time rounded in a dict
	// so that 147 becomes 150, 3432 becomes 3400 and 58760 becomes 59000
	// see also locust's stats.py
	if responseTime < 100 {
		roundedResponseTime = responseTime
	} else if responseTime < 1000 {
		roundedResponseTime = int64(round(float64(responseTime), .5, -1))
	} else if responseTime < 10000 {
		roundedResponseTime = int64(round(float64(responseTime), .5, -2))
	} else {
		roundedResponseTime = int64(round(float64(responseTime), .5, -3))
	}

	_, ok := s.responseTimes[roundedResponseTime]
	if !ok {
		s.responseTimes[roundedResponseTime] = 1
	} else {
		s.responseTimes[roundedResponseTime]++
	}
}

func (s *statsEntry) logError(err string) {
	s.numFailures++
}

func (s *statsEntry) serialize() map[string]interface{} {
	return map[string]interface{}{
		"name":                   s.name,
		"method":                 s.method,
		"last_request_timestamp": s.lastRequestTimestamp,
		"start_time":             s.startTime,
		"num_requests":           s.numRequests,
		"num_failures":           s.numFailures,
		"total_response_time":    s.totalResponseTime,
		"max_response_time":      s.maxResponseTime,
		"min_response_time":      s.minResponseTime,
		"total_content_length":   s.totalContentLength,
		"response_times":         s.responseTimes,
		"num_reqs_per_sec":       s.numReqsPerSec,
	}
}

func (s *statsEntry) getStrippedReport() map[string]interface{} {
	report := s.serialize()
	s.reset()
	return report
}

type statsError struct {
	name       string
	method     string
	error      string
	occurences int64
}

func (err *statsError) occured() {
	err.occurences++
}

func (err *statsError) toMap() map[string]interface{} {
	return map[string]interface{}{
		"method":     err.method,
		"name":       err.name,
		"error":      err.error,
		"occurences": err.occurences,
	}
}

func collectReportData() map[string]interface{} {
	data := map[string]interface{}{
		"stats":       stats.serializeStats(),
		"stats_total": stats.total.getStrippedReport(),
		"errors":      stats.serializeErrors(),
	}
	stats.errors = make(map[string]*statsError)
	return data
}
