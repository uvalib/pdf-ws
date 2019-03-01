package main

import (
	"flag"
	"os"
	"strconv"
)

type configItem struct {
	flag string
	env  string
	desc string
}

type configStringItem struct {
	value string
	configItem
}

type configBoolItem struct {
	value bool
	configItem
}

type configData struct {
	listenPort               configStringItem
	dbHost                   configStringItem
	dbName                   configStringItem
	dbUser                   configStringItem
	dbPass                   configStringItem
	dbAllowOldPasswords      configBoolItem
	tsApiHost                configStringItem
	tsApiGetPidTemplate      configStringItem
	tsApiGetManifestTemplate configStringItem
	jp2kDir                  configStringItem
	archiveDir               configStringItem
	storageDir               configStringItem
	scriptDir                configStringItem
	templateDir              configStringItem
	allowUnpublished         configBoolItem
	iiifUrlTemplate          configStringItem
}

var config configData

func init() {
	config.listenPort = configStringItem{value: "", configItem: configItem{flag: "l", env: "PDFWS_LISTEN_PORT", desc: "listen port"}}
	config.dbHost = configStringItem{value: "", configItem: configItem{flag: "h", env: "PDFWS_DB_HOST", desc: "database host"}}
	config.dbName = configStringItem{value: "", configItem: configItem{flag: "n", env: "PDFWS_DB_NAME", desc: "database name"}}
	config.dbUser = configStringItem{value: "", configItem: configItem{flag: "u", env: "PDFWS_DB_USER", desc: "database user"}}
	config.dbPass = configStringItem{value: "", configItem: configItem{flag: "p", env: "PDFWS_DB_PASS", desc: "database password"}}
	config.dbAllowOldPasswords = configBoolItem{value: false, configItem: configItem{flag: "o", env: "PDFWS_DB_ALLOW_OLD_PASSWORDS", desc: "allow old database passwords"}}
	config.tsApiHost = configStringItem{value: "", configItem: configItem{flag: "H", env: "PDFWS_TRACKSYS_API_HOST", desc: "tracksys host"}}
	config.tsApiGetPidTemplate = configStringItem{value: "", configItem: configItem{flag: "P", env: "PDFWS_TRACKSYS_API_GET_PID_TEMPLATE", desc: "tracksys api get pid template"}}
	config.tsApiGetManifestTemplate = configStringItem{value: "", configItem: configItem{flag: "M", env: "PDFWS_TRACKSYS_API_GET_MANIFEST_TEMPLATE", desc: "tracksys api get manifest template"}}
	config.jp2kDir = configStringItem{value: "", configItem: configItem{flag: "j", env: "PDFWS_JP2K_DIR", desc: "jp2k directory"}}
	config.archiveDir = configStringItem{value: "", configItem: configItem{flag: "m", env: "PDFWS_ARCHIVE_DIR", desc: "archival tif mount directory"}}
	config.storageDir = configStringItem{value: "", configItem: configItem{flag: "t", env: "PDFWS_PDF_STORAGE_DIR", desc: "pdf storage directory"}}
	config.scriptDir = configStringItem{value: "", configItem: configItem{flag: "r", env: "PDFWS_SCRIPT_DIR", desc: "helper script directory"}}
	config.templateDir = configStringItem{value: "", configItem: configItem{flag: "w", env: "PDFWS_WEB_TEMPLATE_DIR", desc: "web template directory"}}
	config.allowUnpublished = configBoolItem{value: false, configItem: configItem{flag: "a", env: "PDFWS_ALLOW_UNPUBLISHED", desc: "allow unpublished"}}
	config.iiifUrlTemplate = configStringItem{value: "", configItem: configItem{flag: "i", env: "PDFWS_IIIF_URL_TEMPLATE", desc: "iiif url template"}}
}

func getBoolEnv(optEnv string) bool {
	value, _ := strconv.ParseBool(os.Getenv(optEnv))

	return value
}

func ensureConfigStringSet(item *configStringItem) bool {
	isSet := true

	if item.value == "" {
		isSet = false
		logger.Printf("[ERROR] %s is not set, use %s variable or -%s flag", item.desc, item.env, item.flag)
	}

	return isSet
}

func flagStringVar(item *configStringItem) {
	flag.StringVar(&item.value, item.flag, os.Getenv(item.env), item.desc)
}

func flagBoolVar(item *configBoolItem) {
	flag.BoolVar(&item.value, item.flag, getBoolEnv(item.env), item.desc)
}

func getConfigValues() {
	// get values from the command line first, falling back to environment variables
	flagStringVar(&config.listenPort)
	flagStringVar(&config.dbHost)
	flagStringVar(&config.dbName)
	flagStringVar(&config.dbUser)
	flagStringVar(&config.dbPass)
	flagBoolVar(&config.dbAllowOldPasswords)
	flagStringVar(&config.tsApiHost)
	flagStringVar(&config.tsApiGetPidTemplate)
	flagStringVar(&config.tsApiGetManifestTemplate)
	flagStringVar(&config.jp2kDir)
	flagStringVar(&config.archiveDir)
	flagStringVar(&config.storageDir)
	flagStringVar(&config.scriptDir)
	flagStringVar(&config.templateDir)
	flagBoolVar(&config.allowUnpublished)
	flagStringVar(&config.iiifUrlTemplate)

	flag.Parse()

	// check each required option, displaying a warning for empty values.
	// die if any of them are not set
	configOK := true
	configOK = ensureConfigStringSet(&config.listenPort) && configOK
	configOK = ensureConfigStringSet(&config.dbHost) && configOK
	configOK = ensureConfigStringSet(&config.dbName) && configOK
	configOK = ensureConfigStringSet(&config.dbUser) && configOK
	configOK = ensureConfigStringSet(&config.dbPass) && configOK
	configOK = ensureConfigStringSet(&config.jp2kDir) && configOK
	configOK = ensureConfigStringSet(&config.archiveDir) && configOK
	configOK = ensureConfigStringSet(&config.storageDir) && configOK
	configOK = ensureConfigStringSet(&config.scriptDir) && configOK
	configOK = ensureConfigStringSet(&config.templateDir) && configOK
	configOK = ensureConfigStringSet(&config.iiifUrlTemplate) && configOK

	if configOK == false {
		flag.Usage()
		os.Exit(1)
	}

	logger.Printf("[CONFIG] listenPort                = [%s]", config.listenPort.value)
	logger.Printf("[CONFIG] dbHost                    = [%s]", config.dbHost.value)
	logger.Printf("[CONFIG] dbName                    = [%s]", config.dbName.value)
	logger.Printf("[CONFIG] dbUser                    = [%s]", config.dbUser.value)
	logger.Printf("[CONFIG] dbPass                    = [REDACTED]")
	logger.Printf("[CONFIG] dbAllowOldPasswords       = [%s]", strconv.FormatBool(config.dbAllowOldPasswords.value))
	logger.Printf("[CONFIG] tsApiHost                 = [%s]", config.tsApiHost.value)
	logger.Printf("[CONFIG] tsApiGetPidTemplate       = [%s]", config.tsApiGetPidTemplate.value)
	logger.Printf("[CONFIG] tsApiGetManifestTemplate  = [%s]", config.tsApiGetManifestTemplate.value)
	logger.Printf("[CONFIG] jp2kDir                   = [%s]", config.jp2kDir.value)
	logger.Printf("[CONFIG] archiveDir                = [%s]", config.archiveDir.value)
	logger.Printf("[CONFIG] storageDir                = [%s]", config.storageDir.value)
	logger.Printf("[CONFIG] scriptDir                 = [%s]", config.scriptDir.value)
	logger.Printf("[CONFIG] templateDir               = [%s]", config.templateDir.value)
	logger.Printf("[CONFIG] allowUnpublished          = [%s]", strconv.FormatBool(config.allowUnpublished.value))
	logger.Printf("[CONFIG] iiifUrlTemplate           = [%s]", config.iiifUrlTemplate.value)
}
