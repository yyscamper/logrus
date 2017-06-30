# Logrus <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/>&nbsp;[![Build Status](https://travis-ci.org/sirupsen/logrus.svg?branch=master)](https://travis-ci.org/sirupsen/logrus)&nbsp;[![GoDoc](https://godoc.org/github.com/sirupsen/logrus?status.svg)](https://godoc.org/github.com/sirupsen/logrus)

Logrus is a structured logger for Go (golang), completely API compatible with
the standard library logger.

This repository is based on the [Logrus](https://github.com/sirupsen/logrus) with some customization, it is compability with the official Logrus, for detail usage about Logrus, please go to this link [Logrus](https://github.com/sirupsen/logrus).

Below are the list of all my customization:
## Module Level Logging
A project often contains a lot of modules, when we turn on the debug message, actually all debug message from all modules will be printed and flooded the console. I want to only output the debug message for some particular modules and still keep other modules operate on the default logging level. The "Module Level Logging" is designed for this.
```go
fooLogger := logrus.NewModule("foo")
fooLogger.Debug("some message")

barLogger := logrus.NewModule("bar")
barLogger.Debug("some other message")

logrus.SetModuleLevel("foo", logrus.LevelDebug)
logrus.SetModuleLevel("bar", logrus.LevelInfo)

//You can set the logging level for different modules in following convenient way:
// For example: Set module "foo" in debug level, and "bar" in error level:
SetModuleLevelString("foo:debug, bar:error") //"foo:debug; bar:error" is also OK
                                             //"foo:debug | bar:error" is also OK
// Set module "foo" in debug level, and "bar" in error level, and all others in info level:
SetModuleLevelString("foo:debug, bar:error, *:info")
// Set all modules to info level:
SetModuleLevelString("*:info")
// or even more simple:
SetModuleLevelString("info")
```


## New Logging Level "Trace"

```go
logrus.Trace("foo")
logrus.WithField("test", 123).Tracef("name=%s", "yyscamper")
```

## Stacktrace
```go
logrus.WithStacktrace().Debug("something happens")

//Set to automatically collect stacktrace if has error
logrus.SetStacktraceOnError(true)
logrus.WithError(err).Error("file is not found")
```

## PrettyTextFormatter
GO's default formatter for map and structure is ugly, I find a pretty formatter [davecgh/go-spew](https://github.com/davecgh/go-spew). To incorporate into logrus, I forked it and also did a little customization, see my repo [yyscamper/go-spew](https://github.com/yyscamper/go-spew) for detail.

Base on `go-spew`, I created a new formatter for loggrus `PrettyTextFormatter`, it outputs the data in a pretty style.

```go
logrus.SetFormatter(&log.PrettyTextFormatter{})
```
