package configs

import "time"

//Config 项目配置
type AllConfig struct {
	Server   ServerConfig
	Logger   LoggerConfig
	Db       DbConfig
	Cache    CacheConfig
	Template TemplateConfig
	Route    RouteConfig
	Redis    RedisConfig
	Session  SessionConfig
	Custom   map[string]string
}

type RedisConfig struct {
	Server     string
	Prefix     string
	Expiration int32
	Db         int
	Password   string
}

type SessionConfig struct {
	CookieName string
	ReName     bool
	CacheType  string
}

//ServerConfig ..
type ServerConfig struct {
	Address string
	//Port已废弃，用Address代替
	Port string
	//并发数量限制
	Concurrence   uint
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	HTTPSCertFile string
	HTTPSKeyFile  string
	HTTPSPhrase   string
}

//LoggerConfig ..
type LoggerConfig struct {
	LogGoID   bool
	LogFile   string
	LogLevel  string
	IsConsole bool
	LogType   string
	LogMaxNum int32
	LogSize   int64
	LogUnit   string
}

//DbConfig ..
type DbConfig struct {
	Driver       string
	Username     string
	Password     string
	Protocol     string
	Address      string
	Port         string
	Dbname       string
	Params       string
	CacheType    string
	MaxIdleConns int
	MaxOpenConns int
	KeepAlive    time.Duration
}

//CacheConfig ..
type CacheConfig struct {
	Type    string
	Servers []string
	Config  struct {
		Prefix     string
		Expiration int32
	}
}

//TemplateConfig ..
type TemplateConfig struct {
	StaticPath  string
	HTMLPath    string
	WidgetsPath string
	IsCache     bool
}

//RouteConfig ..
type RouteConfig struct {
	DefaultController string
	DefaultAction     string
}
