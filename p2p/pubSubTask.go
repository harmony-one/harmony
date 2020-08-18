package p2p

import "fmt"

type pubSubTask interface {
	String() string
	Error() error
}

type addHandlerTask struct {
	handler PubSubHandler
	errC    chan error
}

// String string the string presentation
func (task *addHandlerTask) String() string {
	return fmt.Sprintf("add handler %v", task.handler.Specifier())
}

// Error return the underlying error of the task.
func (task *addHandlerTask) Error() error {
	return <-task.errC
}

type stopHandlerTask struct {
	spec string
	errC chan error
}

// String string the string presentation
func (task *stopHandlerTask) String() string {
	return fmt.Sprintf("stop handler %v", task.spec)
}

// Error return the underlying error of the task.
func (task *stopHandlerTask) Error() error {
	return <-task.errC
}

type stopTopicTask struct {
	errC chan error
}

// String string the string presentation
func (task *stopTopicTask) String() string {
	return fmt.Sprintf("stopping topic")
}

// Error return the underlying error of the task.
func (task *stopTopicTask) Error() error {
	return <-task.errC
}
