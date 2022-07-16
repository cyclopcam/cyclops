package log

// PrefixLogger writes to the underlying log, but all messages are prefixed with a string of your choice
type PrefixLogger struct {
	Log    Log
	Prefix string
}

// Create a new PrefixLogger
func NewPrefixLogger(log Log, prefix string) *PrefixLogger {
	return NewPrefixLoggerNoSpace(log, prefix+" ")
}

// Create a new PrefixLogger, but don't add a space onto 'prefix'
func NewPrefixLoggerNoSpace(log Log, prefix string) *PrefixLogger {
	return &PrefixLogger{
		Log:    log,
		Prefix: prefix,
	}
}

func (l *PrefixLogger) Close() {
	l.Log.Close()
}

func (l *PrefixLogger) Debugf(format string, a ...interface{}) {
	l.Log.Debugf(l.Prefix+format, a...)
}

func (l *PrefixLogger) Infof(format string, a ...interface{}) {
	l.Log.Infof(l.Prefix+format, a...)
}

func (l *PrefixLogger) Warnf(format string, a ...interface{}) {
	l.Log.Warnf(l.Prefix+format, a...)
}

func (l *PrefixLogger) Errorf(format string, a ...interface{}) {
	l.Log.Errorf(l.Prefix+format, a...)
}

func (l *PrefixLogger) Criticalf(format string, a ...interface{}) {
	l.Log.Criticalf(l.Prefix+format, a...)
}
