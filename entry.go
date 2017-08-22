package logrus

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/yyscamper/errors"
	"github.com/yyscamper/go-spew/spew"
)

const (
	DefaultModuleName string = ""
)

var bufferPool *sync.Pool

func init() {
	bufferPool = &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
}

// Defines the key when adding errors using WithError.
var ErrorKey = "error"
var StacktraceKey = "stacktrace"
var ModuleNameKey = "module"

// An entry is the final or intermediate Logrus logging entry. It contains all
// the fields passed with WithField{,s}. It's finally logged when Debug, Info,
// Warn, Error, Fatal or Panic is called on it. These objects can be reused and
// passed around as much as you wish to avoid field duplication.
type Entry struct {
	Logger *Logger

	// Contains all the fields set by the user.
	Data Fields

	// Time at which the log entry was created
	Time time.Time

	// Level the log entry was logged at: Debug, Info, Warn, Error, Fatal or Panic
	Level Level

	// Message passed to Debug, Info, Warn, Error, Fatal or Panic
	Message string

	// When formatter is called in entry.log(), an Buffer may be set to entry
	Buffer *bytes.Buffer
}

func NewEntry(logger *Logger) *Entry {
	return &Entry{
		Logger: logger,
		// Default is three fields, give a little extra room
		Data: make(Fields, 5),
	}
}

// Returns the string representation from the reader and ultimately the
// formatter.
func (entry *Entry) String() (string, error) {
	serialized, err := entry.Logger.Formatter.Format(entry)
	if err != nil {
		return "", err
	}
	str := string(serialized)
	return str, nil
}

func (entry *Entry) withStack(trace string) *Entry {
	return entry.WithField(StacktraceKey, trace)
}

// Add current stacktrace to the Entry
func (entry *Entry) WithStack() *Entry {
	return entry.withStack(errors.Stack(1))
}

// Add an error as single field (using the key defined in ErrorKey) to the Entry.
func (entry *Entry) WithError(err error) *Entry {
	switch realErr := err.(type) {
	case *errors.Error:
		fields := Fields{
			ErrorKey:      err,
			StacktraceKey: realErr.Stack(),
		}
		if realErr.Name != "" {
			fields[ModuleNameKey] = realErr.Name
		}
		for k, v := range realErr.Fields {
			fields[k] = v
		}
		return entry.WithFields(fields)
	default:
		fields := Fields{
			ErrorKey:      err,
			StacktraceKey: errors.Stack(2),
		}
		return entry.WithFields(fields)
	}
}

// Add a single field to the Entry.
func (entry *Entry) WithField(key string, value interface{}) *Entry {
	return entry.WithFields(Fields{key: value})
}

// Add a map of fields to the Entry.
func (entry *Entry) WithFields(fields Fields) *Entry {
	data := make(Fields, len(entry.Data)+len(fields))
	for k, v := range entry.Data {
		data[k] = v
	}
	for k, v := range fields {
		data[k] = v
	}
	return &Entry{Logger: entry.Logger, Data: data}
}

func stringify(val interface{}) string {
	if k, ok := val.(string); ok {
		return k
	}
	return fmt.Sprintf("%v", val)
}

// Add one or multiple field to the Entry.
func (entry *Entry) With(key string, value interface{}, extras ...interface{}) *Entry {
	fields := Fields{}
	fields[key] = value

	n := len(extras)
	if n%2 != 0 {
		n--
	}
	for i := 0; i < n; i += 2 {
		fields[stringify(extras[i])] = extras[i+1]
	}
	//Auto attaches a value if user forgets the last value
	if n < len(extras) {
		fields[stringify(extras[n])] = nil
	}
	return entry.WithFields(fields)
}

func (entry *Entry) WithModule(moduleName string) *Entry {
	return entry.WithField(ModuleNameKey, moduleName)
}

// This function is not declared with a pointer value because otherwise
// race conditions will occur when using multiple goroutines
func (entry Entry) log(level Level, msg string) {
	var buffer *bytes.Buffer
	entry.Time = time.Now()
	entry.Level = level
	entry.Message = msg

	if err := entry.Logger.Hooks.Fire(level, &entry); err != nil {
		entry.Logger.mu.Lock()
		fmt.Fprintf(os.Stderr, "Failed to fire hook: %v\n", err)
		entry.Logger.mu.Unlock()
	}
	buffer = bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer bufferPool.Put(buffer)
	entry.Buffer = buffer
	serialized, err := entry.Logger.Formatter.Format(&entry)
	entry.Buffer = nil
	if err != nil {
		entry.Logger.mu.Lock()
		fmt.Fprintf(os.Stderr, "Failed to obtain reader, %v\n", err)
		entry.Logger.mu.Unlock()
	} else {
		entry.Logger.mu.Lock()
		_, err = entry.Logger.Out.Write(serialized)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
		}
		entry.Logger.mu.Unlock()
	}

	// To avoid Entry#log() returning a value that only would make sense for
	// panic() to use in Entry#Panic(), we avoid the allocation by checking
	// directly here.
	if level <= PanicLevel {
		panic(&entry)
	}
}

func (entry *Entry) Trace(args ...interface{}) {
	if entry.matchLevel(TraceLevel) {
		entry.log(TraceLevel, spew.Sdump(args...))
	}
}

func (entry *Entry) Debug(args ...interface{}) {
	if entry.matchLevel(DebugLevel) {
		entry.log(DebugLevel, spew.Sdump(args...))
	}
}

func (entry *Entry) Print(args ...interface{}) {
	entry.Info(args...)
}

func (entry *Entry) Info(args ...interface{}) {
	if entry.matchLevel(InfoLevel) {
		entry.log(InfoLevel, spew.Sdump(args...))
	}
}

func (entry *Entry) Warn(args ...interface{}) {
	if entry.matchLevel(WarnLevel) {
		entry.log(WarnLevel, spew.Sdump(args...))
	}
}

func (entry *Entry) Warning(args ...interface{}) {
	entry.Warn(args...)
}

func (entry *Entry) Error(args ...interface{}) {
	if entry.matchLevel(ErrorLevel) {
		entry.log(ErrorLevel, spew.Sdump(args...))
	}
}

func (entry *Entry) Fatal(args ...interface{}) {
	if entry.matchLevel(FatalLevel) {
		entry.log(FatalLevel, spew.Sdump(args...))
	}
	Exit(1)
}

func (entry *Entry) Panic(args ...interface{}) {
	if entry.matchLevel(PanicLevel) {
		entry.log(PanicLevel, spew.Sdump(args...))
	}
	panic(spew.Sdump(args...))
}

// Entry Printf family functions

func (entry *Entry) Tracef(format string, args ...interface{}) {
	if entry.matchLevel(TraceLevel) {
		entry.Trace(spew.Sprintf(format, args...))
	}
}

func (entry *Entry) Debugf(format string, args ...interface{}) {
	if entry.matchLevel(DebugLevel) {
		entry.Debug(fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Infof(format string, args ...interface{}) {
	if entry.matchLevel(InfoLevel) {
		entry.Info(fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Printf(format string, args ...interface{}) {
	entry.Infof(format, args...)
}

func (entry *Entry) Warnf(format string, args ...interface{}) {
	if entry.matchLevel(WarnLevel) {
		entry.Warn(fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Warningf(format string, args ...interface{}) {
	entry.Warnf(format, args...)
}

func (entry *Entry) Errorf(format string, args ...interface{}) {
	if entry.matchLevel(ErrorLevel) {
		entry.Error(fmt.Sprintf(format, args...))
	}
}

func (entry *Entry) Fatalf(format string, args ...interface{}) {
	if entry.matchLevel(FatalLevel) {
		entry.Fatal(fmt.Sprintf(format, args...))
	}
	Exit(1)
}

func (entry *Entry) Panicf(format string, args ...interface{}) {
	if entry.matchLevel(PanicLevel) {
		entry.Panic(fmt.Sprintf(format, args...))
	}
}

// Entry Println family functions

func (entry *Entry) Traceln(args ...interface{}) {
	if entry.matchLevel(TraceLevel) {
		entry.Trace(entry.sprintlnn(args...))
	}
}

func (entry *Entry) Debugln(args ...interface{}) {
	if entry.matchLevel(DebugLevel) {
		entry.Debug(entry.sprintlnn(args...))
	}
}

func (entry *Entry) Infoln(args ...interface{}) {
	if entry.matchLevel(InfoLevel) {
		entry.Info(entry.sprintlnn(args...))
	}
}

func (entry *Entry) Println(args ...interface{}) {
	entry.Infoln(args...)
}

func (entry *Entry) Warnln(args ...interface{}) {
	if entry.matchLevel(WarnLevel) {
		entry.Warn(entry.sprintlnn(args...))
	}
}

func (entry *Entry) Warningln(args ...interface{}) {
	entry.Warnln(args...)
}

func (entry *Entry) Errorln(args ...interface{}) {
	if entry.matchLevel(ErrorLevel) {
		entry.Error(entry.sprintlnn(args...))
	}
}

func (entry *Entry) Fatalln(args ...interface{}) {
	if entry.matchLevel(FatalLevel) {
		entry.Fatal(entry.sprintlnn(args...))
	}
	Exit(1)
}

func (entry *Entry) Panicln(args ...interface{}) {
	if entry.matchLevel(PanicLevel) {
		entry.Panic(entry.sprintlnn(args...))
	}
}

// Sprintlnn => Sprint no newline. This is to get the behavior of how
// fmt.Sprintln where spaces are always added between operands, regardless of
// their type. Instead of vendoring the Sprintln implementation to spare a
// string allocation, we do the simplest thing.
func (entry *Entry) sprintlnn(args ...interface{}) string {
	buf := &bytes.Buffer{}
	spew.Fdump(buf, args...)
	buf.WriteString("\n")
	msg := buf.String()
	return msg[:len(msg)-1]
}

func (entry *Entry) matchLevel(lv Level) bool {
	moduleName := DefaultModuleName
	if entry.Data != nil {
		if name, ok := entry.Data[ModuleNameKey]; ok {
			moduleName = name.(string)
		}
	}
	return entry.Logger.level(moduleName) >= lv
}

func (entry *Entry) NewErrorGenerator() *errors.Generator {
	name, ok := entry.Data[ModuleNameKey].(string)
	if !ok {
		name = DefaultModuleName
	}

	errFields := errors.Fields{}
	for k, v := range entry.Data {
		errFields[k] = v
	}
	return errors.NewGenerator(name).WithFields(errFields)
}
