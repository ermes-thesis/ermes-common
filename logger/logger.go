package logger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger interface {
	Close() error
	Debug(msg string, attrs ...any)
	Debugf(format string, args ...any)
	DebugfWith(format string, args []any, attrs ...any)
	Info(msg string, attrs ...any)
	Infof(format string, args ...any)
	InfofWith(format string, args []any, attrs ...any)
	Warn(msg string, attrs ...any)
	Warnf(format string, args ...any)
	WarnfWith(format string, args []any, attrs ...any)
	Error(msg string, attrs ...any)
	Errorf(format string, args ...any)
	ErrorfErr(format string, args ...any) error
	ErrorfWith(format string, args []any, attrs ...any)
	ErrorfErrWith(format string, args []any, attrs ...any) error
	Eval(msg string, attrs ...any)
	Evalf(format string, args ...any)
	EvalfWith(format string, args []any, attrs ...any)

	Enabled(level zerolog.Level) bool
	Mute()
	Unmute()

	With(args ...any) Logger
	Component(name string) Logger
	ComponentWithFile(name string) (Logger, error)
}

type outputEntry struct {
	writer   diode.Writer
	isStdout bool
}

type Impl struct {
	logger       zerolog.Logger
	outputs      []outputEntry
	config       Config
	logFile      io.WriteCloser
	childLoggers []Logger
}

type Config struct {
	Level         Level    `yaml:"level"`
	Format        string   `yaml:"format"` // "text" or "json"
	Output        *LogFile `yaml:"output"`
	StdoutEnabled bool     `yaml:"stdoutEnabled"`
	AddSource     bool     `yaml:"addSource"`
	Colored       bool     `yaml:"colored"`
	Rolling       bool     `yaml:"rolling"`
	Muted         bool     `yaml:"muted"`
}

type LogFile struct {
	LogDir      string `yaml:"logDir"`
	LogFileName string `yaml:"logFileName"`
}

const levelSectionWidthWithColors = 16
const levelSectionWidthWithoutColors = 7

var projectRoot string
var projectRootOnce sync.Once

func makeRotatingFileWriter(logPath string) io.WriteCloser {
	return &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     7,
		Compress:   true,
	}
}

func NewLogger(config Config) (Logger, error) {
	var (
		outputs []outputEntry
		logFile io.WriteCloser
		err     error
	)

	// Checking if an output .log file has been configured
	if config.Output != nil {
		logPath := filepath.Join(config.Output.LogDir, config.Output.LogFileName)
		err = os.MkdirAll(filepath.Dir(logPath), 0755)
		if err != nil {
			return nil, err
		}

		if config.Rolling {
			// If rolling is enabled, we use lumberjack to handle log rotation
			logFile = makeRotatingFileWriter(logPath)
		} else {
			logFile, err = os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
			if err != nil {
				return nil, err
			}
		}

		// If everything went well, add the file to the outputs to which the logger must write
		outputs = append(outputs, outputEntry{
			writer:   buildWriter(config.Format, logFile, true),
			isStdout: false,
		})
	}

	// If an output file has not been configured, fallback to stdout
	// If an output file has been configured, add stdout only if explicitly requested
	if len(outputs) == 0 || config.StdoutEnabled {
		outputs = append(outputs, outputEntry{
			writer:   buildConsoleWriter(os.Stdout, !config.Colored),
			isStdout: true,
		})
	}

	// Notify zerolog to use multiple writers
	var outs []io.Writer
	for _, o := range outputs {
		outs = append(outs, o.writer)
	}
	writer := zerolog.MultiLevelWriter(outs...)

	// Initializing the base structure of the logger with a timestamp
	base := zerolog.New(writer).With().Timestamp()

	// If requested add the Caller file:line
	if config.AddSource {
		base = base.CallerWithSkipFrameCount(3)
		zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
			projectRootOnce.Do(func() {
				// Find "ermes-" prefix in the path
				parts := strings.Split(filepath.ToSlash(file), "/")
				for i, part := range parts {
					if strings.HasPrefix(part, "ermes-") {
						projectRoot = filepath.Join(parts[:i+1]...)
						if !filepath.IsAbs(projectRoot) {
							projectRoot = string(filepath.Separator) + projectRoot
						}
						return
					}
				}
			})

			if projectRoot != "" {
				if rel, err := filepath.Rel(projectRoot, file); err == nil {
					return fmt.Sprintf("%s:%d", rel, line)
				}
			}

			return fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}
	}

	// Create the logger
	zLevel, err := parseToZerologLvl(config.Level)
	if err != nil {
		return nil, err
	}
	logger := base.Logger().Level(zLevel)
	return &Impl{
		logger:  logger,
		outputs: outputs,
		config:  config,
		logFile: logFile,
	}, nil
}

// buildConsoleWriter wraps the parameter writer into a zerolog.ConsoleWriter to
// create some pretty logging. This is the default writer that is used to write to
// os.Stdout
func buildConsoleWriter(out io.Writer, noColor bool) diode.Writer {
	return diode.NewWriter(zerolog.ConsoleWriter{
		Out:           out,
		NoColor:       noColor,
		TimeFormat:    "2006-01-02 15:04:05",
		FieldsExclude: []string{"component"},
		PartsOrder:    []string{zerolog.LevelFieldName, zerolog.TimestampFieldName, "component", zerolog.CallerFieldName, zerolog.MessageFieldName},
		FormatPartValueByName: func(i any, s string) string {
			if s == "component" {
				value, ok := i.(string)
				if !ok {
					return ""
				}
				return strings.ToUpper(fmt.Sprintf("[%s]", value))
			}
			return i.(string)
		},
		FormatFieldName: func(i any) string {
			str, ok := i.(string)
			if !ok {
				return ""
			}
			return "| " + str + "="
		},
		FormatLevel: func(levelStr any) string {
			uppercaseLevel := strings.ToUpper(levelStr.(string))
			level := parseLevel(uppercaseLevel)
			levelSection := colorize(fmt.Sprintf("[%s]", uppercaseLevel), levelColors[level], noColor)
			pad := levelSectionWidthWithColors - len(levelSection)
			if noColor {
				pad = levelSectionWidthWithoutColors - len(levelSection)
			}
			levelSection = levelSection + strings.Repeat(" ", pad)
			return levelSection
		},
	}, 1000, 10*time.Millisecond, func(missed int) {
		fmt.Printf("Logger dropped %d messages", missed)
	})
}

// buildWriter is used to build a writer according to the specified format.
//
// If the "json" format is chosen, the parameter writer is returned as it is, and zerolog
// will take care of its formatting (JSON, is the default formatting for zerolog logging)
//
// If the "text" format is chosen, the parameter writer is wrapper in a zerolog.ConsoleWriter
// that will take care of formatting the logging messages in a pretty and easily understandable way
func buildWriter(format string, out io.Writer, noColor bool) diode.Writer {
	if format == "json" {
		return diode.NewWriter(out, 1000, 10*time.Millisecond, func(missed int) {
			fmt.Printf("Logger dropped %d messages", missed)
		})
	}

	return buildConsoleWriter(out, noColor)
}

func (l *Impl) Close() error {
	for _, child := range l.childLoggers {
		err := child.Close()
		if err != nil {
			return err
		}
	}

	for _, o := range l.outputs {
		if o.isStdout {
			continue
		}
		err := o.writer.Close()
		if err != nil {
			return err
		}
	}

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

func (l *Impl) Info(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Info().Fields(attrs).Msg(msg)
	}
}

func (l *Impl) Infof(format string, args ...interface{}) {
	if !l.config.Muted {
		l.logger.Info().Msg(fmt.Sprintf(format, args...))
	}
}

func (l *Impl) InfofWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Info().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Impl) Error(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Error().Fields(attrs).Msg(msg)
	}
}

func (l *Impl) Errorf(format string, args ...interface{}) {
	if !l.config.Muted {
		l.logger.Error().Msgf(format, args...)
	}
}

func (l *Impl) ErrorfErr(format string, args ...interface{}) error {
	errStr := fmt.Sprintf(format, args...)
	if !l.config.Muted {
		l.logger.Error().Msg(errStr)
	}
	return errors.New(errStr)
}

func (l *Impl) Debug(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Debug().Fields(attrs).Msg(msg)
	}
}

func (l *Impl) Debugf(format string, args ...any) {
	if !l.config.Muted {
		l.logger.Debug().Msgf(format, args...)
	}
}

func (l *Impl) DebugfWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Debug().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Impl) Warn(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Warn().Fields(attrs).Msg(msg)
	}
}

func (l *Impl) Warnf(format string, args ...any) {
	if !l.config.Muted {
		l.logger.Warn().Msgf(format, args...)
	}
}

func (l *Impl) WarnfWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Warn().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Impl) ErrorfWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Error().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Impl) ErrorfErrWith(format string, args []any, attrs ...any) error {
	errStr := fmt.Sprintf(format, args...)

	if !l.config.Muted {
		l.logger.Error().Fields(attrs).Msg(errStr)
	}
	return errors.New(errStr)
}

func (l *Impl) Eval(msg string, args ...any) {
	if !l.config.Muted && l.Enabled(zerolog.WarnLevel) {
		// Overwriting the WARN level with EVAL, but treating it still as a WARN
		l.logger.Warn().Str(zerolog.LevelFieldName, "eval").Fields(args).Msg(msg)
	}
}

func (l *Impl) Evalf(format string, args ...any) {
	if !l.config.Muted && l.Enabled(zerolog.WarnLevel) {
		l.logger.Warn().Str(zerolog.LevelFieldName, "eval").Msgf(format, args...)
	}
}

func (l *Impl) EvalfWith(format string, args []any, attrs ...any) {
	if !l.config.Muted && l.Enabled(zerolog.WarnLevel) {
		l.logger.Warn().Str(zerolog.LevelFieldName, "eval").Fields(attrs).Msgf(format, args...)
	}
}

func (l *Impl) Enabled(level zerolog.Level) bool {
	return level >= l.logger.GetLevel()
}

func (l *Impl) Mute() {
	l.config.Muted = true
}

func (l *Impl) Unmute() {
	l.config.Muted = false
}

func (l *Impl) With(args ...any) Logger {
	return &Impl{
		logger: l.logger.With().Fields(args).Logger(),
		config: l.config,
	}
}

func (l *Impl) Component(name string) Logger {
	return &Impl{
		logger: l.logger.With().Str("component", name).Logger(),
		config: l.config,
	}
}

func (l *Impl) ComponentWithFile(name string) (Logger, error) {
	if l.config.Output == nil {
		return nil, fmt.Errorf("cannot create separated log file for component %s: no base log file configured", name)
	}

	newLogPath := filepath.Join(l.config.Output.LogDir, fmt.Sprintf("%s.log", name))

	var logFile io.WriteCloser
	var err error
	if l.config.Rolling {
		// If rolling is enabled, we use lumberjack to handle log rotation
		logFile = makeRotatingFileWriter(newLogPath)
	} else {
		logFile, err = os.OpenFile(newLogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return nil, err
		}
	}
	newWriter := buildWriter(l.config.Format, logFile, true)

	// Notify zerolog to use multiple writers
	var outs []io.Writer
	for _, o := range l.outputs {
		outs = append(outs, o.writer)
	}
	outs = append(outs, newWriter)
	writer := zerolog.MultiLevelWriter(outs...)

	// Initializing the base structure of the logger with a timestamp
	base := zerolog.New(writer).With().Timestamp().Str("component", name)

	// If requested add the Caller file:line
	if l.config.AddSource {
		base = base.CallerWithSkipFrameCount(3)
	}

	// Create the logger
	zLevel, err := zerolog.ParseLevel(l.config.Format)
	if err != nil {
		return nil, err
	}
	logger := base.Logger().Level(zLevel)

	childLogger := &Impl{
		logger: logger,
		outputs: []outputEntry{
			{
				writer:   newWriter,
				isStdout: false,
			},
		},
		config:  l.config,
		logFile: logFile,
	}

	l.childLoggers = append(l.childLoggers, childLogger)

	return childLogger, nil
}
