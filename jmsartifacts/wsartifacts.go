package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	//"launchpad.net/xmlpath"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// By using backticks I create a multi-line string literal
// that is much easier to format.
const helpCopy = `
gogrep [pattern] [filePath]
	- pattern is a regular expression
	- filePath is a file path
 
example:
  $ ./gogrep \(?i\)path ~/.bash_profile
`

func help() {
	fmt.Println(helpCopy)
}

type SIBQueue struct {
	Identifier                 string `xml:"identifier,attr"`
	Reliability                string `xml:"reliability,attr"`
	MaxFailedDeliveries        string `xml:"maxFailedDeliveries,attr"`
	MaintainStrictMessageOrder string `xml:"maintainStrictMessageOrder,attr"`
	ReceiveExclusive           string `xml:"receiveExclusive,attr"`
	DefaultPriority            string `xml:"defaultPriority,attr"`
    Description string `xml:"description,attr"`
}

type SIBDestinationAlias struct {
	Identifier                 string `xml:"identifier,attr"`
    Bus         string `xml:"bus,attr"`
    TargetBus   string `xml:"targetBus,attr"`
    TargetIdentifier string `xml:"targetIdentifier,attr"`
	Reliability                string `xml:"reliability,attr"`
}

type XMIDestinations struct {
	BusName  string
	SIBQueue []SIBQueue `xml:"http://www.ibm.com/websphere/appserver/schemas/6.0/sibresources.xmi SIBQueue"`
    SIBDestinationAlias []SIBDestinationAlias `xml:"http://www.ibm.com/websphere/appserver/schemas/6.0/sibresources.xmi SIBDestinationAlias"`
}

type Properties struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"type,attr"`
	Value    string `xml:"value,attr"`
	Required string `xml:"required,attr"`
}

type J2cAdminObjects struct {
	Name        string       `xml:"name,attr"`
	JndiName    string       `xml:"jndiName,attr"`
	Description string       `xml:"description,attr"`
	Properties  []Properties `xml:"properties"`
}
type J2cActivationSpec struct {
	Name               string               `xml:"name,attr"`
	JndiName           string               `xml:"jndiName,attr"`
	Description        string               `xml:"description,attr"`
	DestJndiName       string               `xml:"destinationJndiName,attr"`
    AuthAlias          string               `xml:"authenticationAlias,attr"`
	ResourceProperties []Properties         `xml:"resourceProperties"`
}

type J2CResourceAdapter struct {
	Name              string              `xml:"name,attr"`
	Factories         []Factories         `xml:"factories"`
	Classpath         string              `xml:"classpath"`
	J2cAdminObjects   []J2cAdminObjects   `xml:"j2cAdminObjects"`
	J2cActivationSpec []J2cActivationSpec `xml:"j2cActivationSpec"`
}

type PropertySet struct {
    Properties []Properties `xml:"resourceProperties"`
}

type ConnectionPool struct {
    ConnTimeout string `xml:"connectionTimeout,attr"`
    MaxConns string `xml:"maxConnections,attr"`
    MinConns string `xml:"minConnections,attr"`
    ReapTime string `xml:"reapTime,attr"`
    UnusedTimeout string `xml:"unusedTimeout,attr"`
    AgedTimeout string `xml:"agedTimeout,attr"`
    PurgePolicy string `xml:"purgePolicy,attr"`
}

type Factories struct {
	Name     string `xml:"name,attr"`
	JndiName string `xml:"jndiName,attr"`
    AuthAlias string `xml:"authDataAlias,attr"`
    PropertySet PropertySet `xml:"propertySet"`
    ConnectionPool ConnectionPool `xml:"connectionPool"`
}

type XMIJMSArtifacts struct {
	ClusterName        string
	J2CResourceAdapter []J2CResourceAdapter `xml:"http://www.ibm.com/websphere/appserver/schemas/5.0/resources.j2c.xmi J2CResourceAdapter"`
}

func DefCloseFile(f *os.File) {
	e := f.Close()
	if e != nil {
		panic(e)
	}
}

func parseBusDestinations(xmi *XMIDestinations, ffName string, outputDir string) {
	//log.Println(ffName)
	r, e := ioutil.ReadFile(ffName)
	if e != nil {
		panic(e)
	}

	_ = xml.Unmarshal([]byte(r), &xmi)

	emkdir := os.MkdirAll(outputDir, 0777)
	if emkdir != nil {
		panic(emkdir)
	}

	fout, err := os.Create(filepath.Join(outputDir, "destinations.txt"))
	if err != nil {
		panic(err)
	}
	rgxp, _ := regexp.Compile(".*_D_SIB") //(.*\\.Bus|.*_D_SIB")
	for _, sibqueue := range xmi.SIBQueue {
		if !rgxp.MatchString(sibqueue.Identifier) {
			fmt.Fprintf(fout, "{ 'action':'create', 'name':'%s', 'additionalParmsList': ['-maxFailedDeliveries', '%s'] },\n", sibqueue.Identifier, sibqueue.MaxFailedDeliveries)
		}
	}

}
func getPropertyValue(ps []Properties, name string, def string) string {
	res := def
	for _, p := range ps {
		if p.Name == name {
			res = p.Value
		}
	}
	return res
}

func parseJmsArtifacts(xmi *XMIJMSArtifacts, ffName string, outputDir string) {
	//log.Println(ffName)
	r, e := ioutil.ReadFile(ffName)
	if e != nil {
		panic(e)
	}

	_ = xml.Unmarshal([]byte(r), &xmi)

	emkdir := os.MkdirAll(outputDir, 0777)
	if emkdir != nil {
		panic(emkdir)
	}

	fout, err := os.Create(filepath.Join(outputDir, "resources.txt"))
	if err != nil {
		panic(err)
	}
	rgxp, _ := regexp.Compile(".*_D_SIB") //(.*\\.Bus|.*_D_SIB")
	for _, j2cra := range xmi.J2CResourceAdapter {
		if j2cra.Name != "SIB JMS Resource Adapter" {
			continue
		}
		for _, jmsqueue := range j2cra.J2cAdminObjects {
			value := getPropertyValue(jmsqueue.Properties, "QueueName", "")
			if !rgxp.MatchString(value) {
				fmt.Fprintf(fout, "{ 'action':'create', 'name':'%s', 'jndiName':'%s', 'queueName':'%s' },\n", jmsqueue.Name, jmsqueue.JndiName, value)
			}
		}
        for _, as := range j2cra.J2cActivationSpec {
            fmt.Fprintf(fout, "\t{ 'action':'create', 'name':'%s', 'jndiName':'%s', 'destinationJndiName':'%s', 'authenticationAlias':'%s', 'destinationType':'Queue' }\n", as.Name, as.JndiName, as.DestJndiName, as.AuthAlias)
        }
	}
}

func parseESBArtifacts(resourceDir string, outputDir string) ([]XMIDestinations, []XMIJMSArtifacts) {
	xmiDestinations := []XMIDestinations{}
	xmiJmsArtifacts := []XMIJMSArtifacts{}
	busesDir := path.Join(resourceDir, "buses")
	d, err := os.Open(busesDir)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer DefCloseFile(d)

	fi, err := d.Readdir(-1)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	for _, fi := range fi {
		if fi.Mode().IsDir() {
			if strings.HasSuffix(fi.Name(), "Bus") {
				xmiDestination := XMIDestinations{BusName: fi.Name()}
				parseBusDestinations(&xmiDestination, filepath.Join(busesDir, fi.Name(), "sib-destinations.xml"), filepath.Join(outputDir, fi.Name()))
				xmiDestinations = append(xmiDestinations, xmiDestination)
			}
		}
	}

	d, err = os.Open(resourceDir)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer DefCloseFile(d)

	fi, err = d.Readdir(-1)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	for _, fi := range fi {
		if fi.Mode().IsDir() && fi.Name() != "buses" {
			xmiJmsArtifact := XMIJMSArtifacts{ClusterName: fi.Name()}
			parseJmsArtifacts(&xmiJmsArtifact, filepath.Join(resourceDir, fi.Name(), "resources.xml"), filepath.Join(outputDir, fi.Name()))
			xmiJmsArtifacts = append(xmiJmsArtifacts, xmiJmsArtifact)
		}
	}
	return xmiDestinations, xmiJmsArtifacts
}

func main() {
	outputDir := "c:/projs/go/src/github.com/msmind/go_battlefield/jmsartifacts2/output"
	resourceDir := "c:/projs/go/src/github.com/msmind/go_battlefield/jmsartifacts2/resources"
	xmiDestinations, xmiJmsArtifacts := parseESBArtifacts(resourceDir, outputDir)
	log.Printf("%#v\n", xmiDestinations)
	log.Printf("%#v\n", xmiJmsArtifacts)

	// parse modules jms artifacts
	modules := []Module{
		{Name: "NF_AccountServiceModule"},
		{Name: "NF_AMLCheckServiceModule"},
		{Name: "NF_CachedDictionaryModule"},
		{Name: "NF_CardInformationServiceModule"},
		{Name: "NF_CFEExchangeModule"},
		{Name: "NF_CapstoneExchangeModule"},
		{Name: "NF_CommonServiceModule"},
		{Name: "NF_D361ExchangeModule"},
		{Name: "NF_D359ExchangeModule"},
		{Name: "NF_DJWatchlistProcessModule"},
		{Name: "NF_DepositServiceModule"},
		{Name: "NF_FactivaExchangeModule"},
		{Name: "NF_IBFacadeModule"},
		{Name: "NF_IBExchangeModule"},
		{Name: "NF_LegalDocumentServiceModule"},
		{Name: "NF_LoanServiceModule"},
		//{Name: "NF_MFMExchangeModule"},
		{Name: "NF_MailNotificationModule"},
		//{Name: "NF_OracleReportExchangeModule"},
		{Name: "NF_PaymentsInfoServiceModule"},
		{Name: "NF_ProfileExchangeModule"},
		{Name: "NF_ReportServiceModule"},
		{Name: "NF_RuntimeEnvironmentModule"},
		{Name: "NF_ScheduleModuleEAR"},
		{Name: "NF_TFEExchangeModule"},
		//{Name: "NF_Way4ExchangeModule"},
		{Name: "LogViewer"},
		{Name: "NFEventConsumerApp"}, // NFEventConsumer
		//{Name: "NF_LoggerModule"},
		{Name: "ResourceViewer"},
		{Name: "ResourceViewerWebModule"},
		{Name: "CSDD361HardRequestsModule"},
		{Name: "CSDD5HardRequestsModule"},
		{Name: "CSDD5LightRequestsModule"},
		{Name: "CSDD5NormRequestsModule"},
		{Name: "CSDFacadeModule"},
		{Name: "CSDIVRModule"},
		{Name: "CSDLoggingModule"},
		{Name: "CSDProfileModule"},
		{Name: "ClD5Module"},
		{Name: "ClInbMedModule"},
		{Name: "ClPfModule"},
		{Name: "IBschedulerModule"},
		{Name: "NF_AvayaExchangeModule"},
		{Name: "NF_CSDExchangeModule"},
		{Name: "NF_CommunicationServiceModule"},
		{Name: "PVIHSMModule"},
		{Name: "PVIModule"},
		{Name: "CDMailNotificationModule"},
		{Name: "FES361LightModule"},
		{Name: "FES368LightModule"},
		{Name: "FESFacadeModule"},
		{Name: "FESLOLightModule"},
		{Name: "FESLoggerModule"},
		{Name: "FESReportModule"},
		{Name: "FESRoute361Module"},
		{Name: "FESRouteModule"},
		{Name: "NF_FESExchangeModule"},
		{Name: "NF_FESFacadeModule"},
		{Name: "NF_VerificationServiceModule"},
		{Name: "ClientSecurityModule"},
		{Name: "CreditCardInfoModule"},
		{Name: "IBProxyModule"},
		{Name: "IVRCustomLoggerModule"},
		{Name: "IVRD5ConLoadSerModule"},
		{Name: "IVRInMedModule"},
		{Name: "IVRMedLegModule"},
		{Name: "IVRSchedulerModule"},
		{Name: "LoanConEmailNotManModule"},
		{Name: "LoanConSMSNotManModule"},
		{Name: "LoanContractInfoModule"},
		{Name: "ProfileSupportModule"},
		{Name: "Way4DBSupportModule"},
	}

	workspaceDir := "c:/projs/RenCredit/nlo.rel-3131.esb/"

	parseModules(workspaceDir, modules)

	mapByJndiNames(xmiDestinations, xmiJmsArtifacts, modules, outputDir)

	emkdir := os.MkdirAll(outputDir, 0777)
	if emkdir != nil {
		panic(emkdir)
	}

	fout, err := os.Create(filepath.Join(outputDir, "modulesartifcats.txt"))
	if err != nil {
		panic(err)
	}
	//rgxp, _ := regexp.Compile(".*_D_SIB") //(.*\\.Bus|.*_D_SIB")
	for _, module := range modules {
		fmt.Fprintf(fout, "%s:\n", module.Name)
		for _, mimport := range module.Import {
			fmt.Fprintf(fout, "\tImports:\n")
			fmt.Fprintf(fout, "\t\t{ 'name':'%s', 'esbBinding':%v },\n", mimport.Name, mimport.EsbBinding)
		}
		for _, mexport := range module.Export {
			fmt.Fprintf(fout, "\tExport:\n")
			fmt.Fprintf(fout, "\t\t{ 'name':'%s', 'esbBinding':%v },\n", mexport.Name, mexport.EsbBinding)
		}
	}
}

func findSIBQueue(xmiDestinations []XMIDestinations, identifier string) (string, SIBQueue) {
	for _, busxmiDestinations := range xmiDestinations {
		for _, sibqueue := range busxmiDestinations.SIBQueue {
			if sibqueue.Identifier == identifier {
				return busxmiDestinations.BusName, sibqueue
			}
		}
	}
	return "", SIBQueue{}
}


func findSIBDestAlias(xmiDestinations []XMIDestinations, identifier string) (string, SIBDestinationAlias) {
	for _, busxmiDestinations := range xmiDestinations {
		for _, sibDestAlias := range busxmiDestinations.SIBDestinationAlias {
			if sibDestAlias.Identifier == identifier {
				return busxmiDestinations.BusName, sibDestAlias
			}
		}
	}
	return "", SIBDestinationAlias{}
}

func findAS(j2cra J2CResourceAdapter, jndiName string) (bool, J2cActivationSpec) {
	for _, j2cAS := range j2cra.J2cActivationSpec {
		if j2cAS.DestJndiName == jndiName {
			return true, j2cAS
		}
	}
	return false, J2cActivationSpec{}
}
func printDestInfo(fout *os.File, xmiJmsArt *XMIJMSArtifacts, sibqueue *SIBQueue) {
    if sibqueue.MaxFailedDeliveries == "" {
        sibqueue.MaxFailedDeliveries = "5"
    }
    fmt.Fprintf(fout, "        { 'action':'create', 'name':'%s', 'additionalParmsList': ['-description', '%s', '-maxFailedDeliveries', '%s'] },\n", sibqueue.Identifier, sibqueue.Description, sibqueue.MaxFailedDeliveries)

}

func mapByJndiNames(xmiDestinations []XMIDestinations, xmiJmsArtifacts []XMIJMSArtifacts, modules []Module, outputDir string) {
	emkdir := os.MkdirAll(outputDir, 0777)
	if emkdir != nil {
		panic(emkdir)
	}

	fout, err := os.Create(filepath.Join(outputDir, "mappedresources.txt"))
	if err != nil {
		panic(err)
	}
    printed := make(map[string]string)

	for _, xmiJmsArt := range xmiJmsArtifacts {
        fmt.Fprintf(fout, "{ 'clustername':'%s' }\n", xmiJmsArt.ClusterName)
		for _, j2cra := range xmiJmsArt.J2CResourceAdapter {
			if j2cra.Name != "SIB JMS Resource Adapter" {
				continue
			}
			for _, j2cAdminObject := range j2cra.J2cAdminObjects {

                value := getPropertyValue(j2cAdminObject.Properties, "QueueName", "")

                busName, sibqueue := findSIBQueue(xmiDestinations, value)
				if existInModule(modules, "queue", j2cAdminObject.JndiName) {
                    if len(sibqueue.Identifier) > 0 {
                        if _, ok := printed[sibqueue.Identifier]; !ok {
                            printDestInfo(fout, &xmiJmsArt, &sibqueue)
                            printed[sibqueue.Identifier] = busName
                        }
                    } else {
                        busName, sibDA := findSIBDestAlias(xmiDestinations, value)
                        if _, ok := printed[sibqueue.Identifier]; !ok {
                            fmt.Fprintf(fout, "        { 'action':'create', 'name':'%s', 'additionalParmsList': ['-reliability', '%s', '-targetBus', '%s', '-targetName', '%s'] },\n", sibDA.Identifier, sibDA.Reliability, sibDA.TargetBus, sibDA.TargetIdentifier)
                            printed[sibDA.Identifier] = busName
                        }
                    }
					fmt.Fprintf(fout, "        { 'action':'create', 'name':'%s', 'jndiName':'%s', 'queueName':'%s' },\n", j2cAdminObject.Name, j2cAdminObject.JndiName, value)

					asfound, as := findAS(j2cra, j2cAdminObject.JndiName)
                    if asfound && existInModule(modules, "as", as.JndiName) {
                        maxConcurrency := getPropertyValue(as.ResourceProperties, "maxConcurrency", "10")
                        fmt.Fprintf(fout, "        { 'action':'create', 'name':'%s', 'jndiName':'%s', 'destinationJndiName':'%s', 'destinationType':'Queue', 'authenticationAlias':'%s', 'additionalParmsList': ['-maxConcurrency', '%s'] },\n", as.Name, as.JndiName, as.DestJndiName, as.AuthAlias, maxConcurrency)
                    }
				} else if len(sibqueue.Identifier) > 0 && sibqueue.Description == "Generated by jython script" {
                    if _, ok := printed[sibqueue.Identifier]; !ok {
                        printDestInfo(fout, &xmiJmsArt, &sibqueue)
                        printed[sibqueue.Identifier] = busName
                    }
                }
			}
            for _, qcf := range j2cra.Factories {
                if existInModule(modules, "cf", qcf.JndiName) {
                    conp := qcf.ConnectionPool
                    //nonPersistentMapping := getPropertyValue(as.ResourceProperties, "NonPersistentMapping", "ExpressNonPersistent")
                    //persistentMapping := getPropertyValue(as.ResourceProperties, "PersistentMapping", "ReliablePersistent")
                    fmt.Fprintf(fout, "        { 'action':'create', 'name':'%s', 'jndiName':'%s', 'type':'queue', 'authAlias':'%s', 'connectionPoolPropsList':['-connectionTimeout', '%s', '-maxConnections', '%s', '-minConnections', '%s', '-reapTime', '%s', '-unusedTimeout', '%s', '-agedTimeout', '%s', '-purgePolicy', '%s'] },\n", qcf.Name, qcf.JndiName, qcf.AuthAlias, conp.ConnTimeout, conp.MaxConns, conp.MinConns, conp.ReapTime, conp.UnusedTimeout, conp.AgedTimeout, conp.PurgePolicy )
                }
            }
		}
	}
}
func existIn(esbBinding EsbBinding, arttype string, jndiName string) bool {
	switch arttype {
	case "queue":
		for _, dest := range esbBinding.Destination {
			if dest.Target == jndiName {
				return true
			}
		}
	case "cf", "as":
		if esbBinding.Connection.Target == jndiName || esbBinding.ResponseConnection.Target == jndiName {
			return true
		}
	}
	return false
}

func existInModule(modules []Module, arttype string, jndiName string) bool {
	for _, module := range modules {
		//fmt.Fprintf(fout, "%s:\n", module.Name)
		for _, mimport := range module.Import {
			if existIn(mimport.EsbBinding, arttype, jndiName) {
				return true
			}
		}
		for _, mexport := range module.Export {
			if existIn(mexport.EsbBinding, arttype, jndiName) {
				return true
			}
		}
	}
	return false
}

type Connection struct {
	Target string `xml:"target,attr"`
}
type Destination struct {
	Target string `xml:"target,attr"`
	Usage  string `xml:"usage,attr"`
}

type EsbBinding struct {
	Type               string        `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
    Endpoint           string       `xml:"endpoint,attr"`
	Connection         Connection    `xml:"connection"`
	ResponseConnection Connection    `xml:"responseConnection"`
	Destination        []Destination `xml:"destination"`
}

type Import struct {
	Name       string     `xml:"name,attr"`
	EsbBinding EsbBinding `xml:"esbBinding"`
}

type Export struct {
	Name       string     `xml:"name,attr"`
	EsbBinding EsbBinding `xml:"esbBinding"`
}

type Module struct {
	Name   string
	Import []Import
	Export []Export
	//ASName       string
	//JmsQueueName string
	//QueueCF      string
}

func parseModuleArtifacts(module *Module, ffName string) {
	//log.Println(ffName)
	r, e := ioutil.ReadFile(ffName)
	if e != nil {
		panic(e)
	}
	if strings.HasSuffix(ffName, ".import") {
		moduleImport := Import{}
		_ = xml.Unmarshal([]byte(r), &moduleImport)
		if strings.HasSuffix(moduleImport.EsbBinding.Type, ":JMSImportBinding") || strings.HasSuffix(moduleImport.EsbBinding.Type, ":WebServiceImportBinding") {
			module.Import = append(module.Import, moduleImport)
			//fmt.Printf("%#v\n", module)
		}
	} else if strings.HasSuffix(ffName, ".export") {
		moduleExport := Export{}
		_ = xml.Unmarshal([]byte(r), &moduleExport)
		if len(moduleExport.EsbBinding.Type)>0 && strings.HasSuffix(moduleExport.EsbBinding.Type, ":JMSExportBinding") {
			module.Export = append(module.Export, moduleExport)
			//fmt.Printf("%#v\n", module)
		}
	}

}

func parseModules(workspaceDir string, modules []Module) {
	for ind, _ := range modules {
		module := &modules[ind]
		moduleDir := filepath.Join(workspaceDir, module.Name)
		log.Println(moduleDir)
		d, err := os.Open(moduleDir)
		if err != nil {
			log.Println(err)
			continue
		}
		defer DefCloseFile(d)

		fi, err := d.Readdir(-1)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, fi := range fi {
			if fi.Mode().IsRegular() {
				match, _ := regexp.MatchString("\\.(import|export)", fi.Name())
				if match {
					parseModuleArtifacts(module, filepath.Join(moduleDir, fi.Name()))
				}
			}
		}

	}
}

func test() {
	// Slicing the arguments passed to this program,
	// starting at position 1. The first argument is
	// always the program name.
	log.Println(os.Args[1:])
	args := os.Args[1:]
	if len(args) < 2 || args[0] == "--help" {
		help()
		os.Exit(1)
	}
	pattern := args[0]
	dirname := args[1]
	if !strings.HasSuffix(dirname, string(filepath.Separator)) {
		dirname = dirname + string(filepath.Separator)
	}
	fmt.Printf("Pattern=%s dir=%s", pattern, dirname)

	// e will not be nil if the regular expresion string fails
	// to compile. An alternative would be regexp.MustCompile()
	// which panics under the same condition.
	reg, e := regexp.Compile(pattern)
	if e != nil {
		log.Println("ERROR [pattern] is not a regex :(")
		help()
		os.Exit(1)
	}

	d, err := os.Open(dirname)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	defer DefCloseFile(d)

	fi, err := d.Readdir(-1)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	f, err := os.OpenFile("c:/projs/go/src/github.com/msmind/go_battlefield/jmsartifacts2/result.txt", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	for _, fi := range fi {
		if fi.Mode().IsDir() && strings.HasSuffix(fi.Name(), "Module") {

			ind, err := os.Open(dirname + fi.Name())
			if err != nil {
				log.Println(err)
				os.Exit(1)
			}

			defer DefCloseFile(ind)

			fj, err := ind.Readdir(-1)
			if err != nil {
				log.Println(err)
				os.Exit(1)
			}
			for _, fj := range fj {
				if !fj.Mode().IsRegular() {
					continue
				}
				// e will not be nil if there is a problem opening the file.
				file, e := os.Open(dirname + fi.Name() + string(filepath.Separator) + fj.Name())
				if e != nil {
					panic(e)
				}

				defer DefCloseFile(file)

				// Scanner will advance an io.Reader, line by line, with each call
				// to Scan(). The value of Text() will be the current line, sans
				// newline.
				scanner := bufio.NewScanner(file)
				lineNumber := 0
				for scanner.Scan() {
					lineNumber += 1
					line := scanner.Text()

					// Returns the index of the leftmost occurance of the regular
					// expression in "line", or nil if the regular expression has
					// no matches.
					submuch := reg.FindStringSubmatch(line)
					if submuch == nil {
						continue
					}
					//fmt.Println(fj.Name(), fj.Size(), "bytes")
					resline := fmt.Sprintf("%s\t%s\n", strings.TrimSpace(submuch[1]), fi.Name())
					fmt.Print(resline)

					if _, err = f.WriteString(resline); err != nil {
						panic(err)
					}
				}
			}
		}
	}
}
