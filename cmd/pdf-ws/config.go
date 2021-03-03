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
	listenPort       configStringItem
	tsAPIHost        configStringItem
	jp2kDir          configStringItem
	archiveDir       configStringItem
	storageDir       configStringItem
	scriptDir        configStringItem
	assetsDir        configStringItem
	templateDir      configStringItem
	iiifURLTemplate  configStringItem
	solrURLTemplate  configStringItem
	virgoURLTemplate configStringItem
}

var config configData

func init() {
	config.listenPort = configStringItem{value: "", configItem: configItem{flag: "l", env: "PDFWS_LISTEN_PORT", desc: "listen port"}}
	config.tsAPIHost = configStringItem{value: "", configItem: configItem{flag: "H", env: "PDFWS_TRACKSYS_API_HOST", desc: "tracksys host"}}
	config.jp2kDir = configStringItem{value: "", configItem: configItem{flag: "j", env: "PDFWS_JP2K_DIR", desc: "jp2k directory"}}
	config.archiveDir = configStringItem{value: "", configItem: configItem{flag: "m", env: "PDFWS_ARCHIVE_DIR", desc: "archival tif mount directory"}}
	config.storageDir = configStringItem{value: "", configItem: configItem{flag: "t", env: "PDFWS_PDF_STORAGE_DIR", desc: "pdf storage directory"}}
	config.scriptDir = configStringItem{value: "", configItem: configItem{flag: "r", env: "PDFWS_SCRIPT_DIR", desc: "helper script directory"}}
	config.assetsDir = configStringItem{value: "", configItem: configItem{flag: "a", env: "PDFWS_ASSETS_DIR", desc: "assets directory"}}
	config.templateDir = configStringItem{value: "", configItem: configItem{flag: "w", env: "PDFWS_WEB_TEMPLATE_DIR", desc: "web template directory"}}
	config.iiifURLTemplate = configStringItem{value: "", configItem: configItem{flag: "i", env: "PDFWS_IIIF_URL_TEMPLATE", desc: "iiif url template"}}
	config.solrURLTemplate = configStringItem{value: "", configItem: configItem{flag: "s", env: "PDFWS_SOLR_URL_TEMPLATE", desc: "solr url template"}}
	config.virgoURLTemplate = configStringItem{value: "", configItem: configItem{flag: "v", env: "PDFWS_VIRGO_URL_TEMPLATE", desc: "virgo url template"}}
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
	flagStringVar(&config.tsAPIHost)
	flagStringVar(&config.jp2kDir)
	flagStringVar(&config.archiveDir)
	flagStringVar(&config.storageDir)
	flagStringVar(&config.scriptDir)
	flagStringVar(&config.assetsDir)
	flagStringVar(&config.templateDir)
	flagStringVar(&config.iiifURLTemplate)
	flagStringVar(&config.solrURLTemplate)
	flagStringVar(&config.virgoURLTemplate)

	flag.Parse()

	// check each required option, displaying a warning for empty values.
	// die if any of them are not set
	configOK := true
	configOK = ensureConfigStringSet(&config.listenPort) && configOK
	configOK = ensureConfigStringSet(&config.jp2kDir) && configOK
	configOK = ensureConfigStringSet(&config.archiveDir) && configOK
	configOK = ensureConfigStringSet(&config.storageDir) && configOK
	configOK = ensureConfigStringSet(&config.scriptDir) && configOK
	configOK = ensureConfigStringSet(&config.assetsDir) && configOK
	configOK = ensureConfigStringSet(&config.templateDir) && configOK
	configOK = ensureConfigStringSet(&config.iiifURLTemplate) && configOK
	configOK = ensureConfigStringSet(&config.solrURLTemplate) && configOK
	configOK = ensureConfigStringSet(&config.virgoURLTemplate) && configOK

	if configOK == false {
		flag.Usage()
		os.Exit(1)
	}

	logger.Printf("[CONFIG] listenPort                = [%s]", config.listenPort.value)
	logger.Printf("[CONFIG] tsAPIHost                 = [%s]", config.tsAPIHost.value)
	logger.Printf("[CONFIG] jp2kDir                   = [%s]", config.jp2kDir.value)
	logger.Printf("[CONFIG] archiveDir                = [%s]", config.archiveDir.value)
	logger.Printf("[CONFIG] storageDir                = [%s]", config.storageDir.value)
	logger.Printf("[CONFIG] scriptDir                 = [%s]", config.scriptDir.value)
	logger.Printf("[CONFIG] assetsDir                 = [%s]", config.assetsDir.value)
	logger.Printf("[CONFIG] templateDir               = [%s]", config.templateDir.value)
	logger.Printf("[CONFIG] iiifURLTemplate           = [%s]", config.iiifURLTemplate.value)
	logger.Printf("[CONFIG] solrURLTemplate           = [%s]", config.solrURLTemplate.value)
	logger.Printf("[CONFIG] virgoURLTemplate          = [%s]", config.virgoURLTemplate.value)
}
