package sttypes

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		bytesReadCounter,
		bytesWriteCounter,
		msgReadCounter,
		msgWriteCounter,
		msgReadFailedCounterVec,
		msgWriteFailedCounterVec,
		recoverableErrorCounterVec,
		criticalErrorCounterVec,
		streamClosedByRecoverableErrorsCounter,
	)
}

var (
	bytesReadCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "bytes_read",
			Help:      "total bytes read from stream",
		},
	)

	bytesWriteCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "bytes_write",
			Help:      "total bytes write to stream",
		},
	)

	msgReadCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "msg_read",
			Help:      "number of messages read from stream",
		},
	)

	msgWriteCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "msg_write",
			Help:      "number of messages write to stream",
		},
	)

	msgReadFailedCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "msg_read_failed",
			Help:      "number of messages failed reading from stream",
		},
		[]string{"error"},
	)

	msgWriteFailedCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "msg_write_failed",
			Help:      "number of messages failed writing to stream",
		},
		[]string{"error"},
	)

	recoverableErrorCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "recoverable_errors_total",
			Help:      "total number of recoverable errors encountered in stream operations",
		},
		[]string{"error_type"},
	)

	criticalErrorCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "critical_errors_total",
			Help:      "total number of critical errors that caused stream closure",
		},
		[]string{"error_type"},
	)

	streamClosedByRecoverableErrorsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "streams_closed_by_recoverable_errors_total",
			Help:      "total number of streams closed due to exceeding max recoverable error retries",
		},
	)
)

// RecordRecoverableError records a recoverable error metric
func RecordRecoverableError(errorType StreamErrorType) {
	recoverableErrorCounterVec.With(prometheus.Labels{
		"error_type": errorType.String(),
	}).Inc()
}

// RecordCriticalError records a critical error metric
func RecordCriticalError(errorType StreamErrorType) {
	criticalErrorCounterVec.With(prometheus.Labels{
		"error_type": errorType.String(),
	}).Inc()
}

// RecordStreamClosedByRecoverableErrors records when a stream is closed due to too many recoverable errors
func RecordStreamClosedByRecoverableErrors() {
	streamClosedByRecoverableErrorsCounter.Inc()
}
