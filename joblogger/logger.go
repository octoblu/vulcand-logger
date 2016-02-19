package logger

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/octoblu/vulcand-logger/wrapper"
)

// Logger holds the oxy circuit breaker.
type Logger struct {
	redisChannel chan []byte
	router       *mux.Router
}

// New returns a new Logger.
func New(RedisURI, QueueName string, router *mux.Router) *Logger {
	redisChannel := make(chan []byte)
	go runLogger(RedisURI, QueueName, redisChannel)

	return &Logger{redisChannel, router}
}

func (logger *Logger) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	startTime := time.Now()
	redisChannel := logger.redisChannel

	backendName := "unknown"

	routeMatch := mux.RouteMatch{}
	if logger.router.Match(r, &routeMatch) {
		backendName = routeMatch.Route.GetName()
	}

	wrapped := wrapper.New(rw, redisChannel, startTime, backendName)
	next(wrapped, r)
}

func logError(fmtMessage string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, fmtMessage, err.Error())
}

func runLogger(redisURI, queueName string, logChannel chan []byte) {
	redisConn, err := redis.DialURL(redisURI)
	logError("redis.DialURL Failed: %v\n", err)

	for {
		logEntryBytes := <-logChannel
		_, err = redisConn.Do("lpush", queueName, logEntryBytes)
		logError("Redis LPUSH failed: %v\n", err)
	}
}