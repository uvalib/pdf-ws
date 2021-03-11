package main

import (
	"flag"
	"log"
	"os"
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

type configData struct {
	listenPort       configStringItem
	tsAPIHost        configStringItem
	storageDir       configStringItem
	scriptDir        configStringItem
	assetsDir        configStringItem
	templateDir      configStringItem
	iiifURLTemplate  configStringItem
	solrURLTemplate  configStringItem
	virgoURLTemplate configStringItem
	pdfChunkSize     configStringItem
}

var config configData

func init() {
	config.listenPort = configStringItem{value: "", configItem: configItem{flag: "l", env: "PDFWS_LISTEN_PORT", desc: "listen port"}}
	config.tsAPIHost = configStringItem{value: "", configItem: configItem{flag: "H", env: "PDFWS_TRACKSYS_API_HOST", desc: "tracksys host"}}
	config.storageDir = configStringItem{value: "", configItem: configItem{flag: "t", env: "PDFWS_PDF_STORAGE_DIR", desc: "pdf storage directory"}}
	config.scriptDir = configStringItem{value: "", configItem: configItem{flag: "r", env: "PDFWS_SCRIPT_DIR", desc: "helper script directory"}}
	config.assetsDir = configStringItem{value: "", configItem: configItem{flag: "a", env: "PDFWS_ASSETS_DIR", desc: "assets directory"}}
	config.templateDir = configStringItem{value: "", configItem: configItem{flag: "w", env: "PDFWS_WEB_TEMPLATE_DIR", desc: "web template directory"}}
	config.iiifURLTemplate = configStringItem{value: "", configItem: configItem{flag: "i", env: "PDFWS_IIIF_URL_TEMPLATE", desc: "iiif url template"}}
	config.solrURLTemplate = configStringItem{value: "", configItem: configItem{flag: "s", env: "PDFWS_SOLR_URL_TEMPLATE", desc: "solr url template"}}
	config.virgoURLTemplate = configStringItem{value: "", configItem: configItem{flag: "v", env: "PDFWS_VIRGO_URL_TEMPLATE", desc: "virgo url template"}}
	config.pdfChunkSize = configStringItem{value: "", configItem: configItem{flag: "c", env: "PDFWS_PDF_CHUNK_SIZE", desc: "pdf chunk size"}}
}

func ensureConfigStringSet(item *configStringItem) bool {
	isSet := true

	if item.value == "" {
		isSet = false
		log.Printf("[ERROR] %s is not set, use %s variable or -%s flag", item.desc, item.env, item.flag)
	}

	return isSet
}

func flagStringVar(item *configStringItem) {
	flag.StringVar(&item.value, item.flag, os.Getenv(item.env), item.desc)
}

func getConfigValues() {
	// get values from the command line first, falling back to environment variables
	flagStringVar(&config.listenPort)
	flagStringVar(&config.tsAPIHost)
	flagStringVar(&config.storageDir)
	flagStringVar(&config.scriptDir)
	flagStringVar(&config.assetsDir)
	flagStringVar(&config.templateDir)
	flagStringVar(&config.iiifURLTemplate)
	flagStringVar(&config.solrURLTemplate)
	flagStringVar(&config.virgoURLTemplate)
	flagStringVar(&config.pdfChunkSize)

	flag.Parse()

	// check each required option, displaying a warning for empty values.
	// die if any of them are not set
	configOK := true
	configOK = ensureConfigStringSet(&config.listenPort) && configOK
	configOK = ensureConfigStringSet(&config.storageDir) && configOK
	configOK = ensureConfigStringSet(&config.scriptDir) && configOK
	configOK = ensureConfigStringSet(&config.assetsDir) && configOK
	configOK = ensureConfigStringSet(&config.templateDir) && configOK
	configOK = ensureConfigStringSet(&config.iiifURLTemplate) && configOK
	configOK = ensureConfigStringSet(&config.solrURLTemplate) && configOK
	configOK = ensureConfigStringSet(&config.virgoURLTemplate) && configOK
	configOK = ensureConfigStringSet(&config.pdfChunkSize) && configOK

	if configOK == false {
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("[CONFIG] listenPort       = [%s]", config.listenPort.value)
	log.Printf("[CONFIG] tsAPIHost        = [%s]", config.tsAPIHost.value)
	log.Printf("[CONFIG] storageDir       = [%s]", config.storageDir.value)
	log.Printf("[CONFIG] scriptDir        = [%s]", config.scriptDir.value)
	log.Printf("[CONFIG] assetsDir        = [%s]", config.assetsDir.value)
	log.Printf("[CONFIG] templateDir      = [%s]", config.templateDir.value)
	log.Printf("[CONFIG] iiifURLTemplate  = [%s]", config.iiifURLTemplate.value)
	log.Printf("[CONFIG] solrURLTemplate  = [%s]", config.solrURLTemplate.value)
	log.Printf("[CONFIG] virgoURLTemplate = [%s]", config.virgoURLTemplate.value)
	log.Printf("[CONFIG] pdfChunkSize     = [%s]", config.pdfChunkSize.value)
}
