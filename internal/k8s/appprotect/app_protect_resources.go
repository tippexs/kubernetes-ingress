package appprotect

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var appProtectPolicyRequiredFields = [][]string{
	{"spec", "policy"},
}

var appProtectLogConfRequiredFields = [][]string{
	{"spec", "content"},
	{"spec", "filter"},
}

var appProtectUserSigRequiredSlices = [][]string{
	{"spec", "signatures"},
}

var appProtectPolicyExtRefs = [][]string{
	{"spec", "policy", "modificationsReference"},
	{"spec", "policy", "blockingSettingReference"},
	{"spec", "policy", "signatureSettingReference"},
	{"spec", "policy", "serverTechnologyReference"},
	{"spec", "policy", "headerReference"},
	{"spec", "policy", "cookieReference"},
	{"spec", "policy", "dataGuardReference"},
	{"spec", "policy", "filetypeReference"},
	{"spec", "policy", "methodReference"},
	{"spec", "policy", "generalReference"},
	{"spec", "policy", "parameterReference"},
	{"spec", "policy", "sensitiveParameterReference"},
	{"spec", "policy", "jsonProfileReference"},
	{"spec", "policy", "xmlProfileReference"},
	{"spec", "policy", "whitelistIpReference"},
	{"spec", "policy", "responsePageReference"},
	{"spec", "policy", "characterSetReference"},
	{"spec", "policy", "cookieSettingsReference"},
	{"spec", "policy", "headerSettingsReference"},
	{"spec", "policy", "jsonValidationFileReference"},
	{"spec", "policy", "xmlValidationFileReference"},
	{"spec", "policy", "signatureSetReference"},
	{"spec", "policy", "signatureReference"},
	{"spec", "policy", "urlReference"},
	{"spec", "policy", "threatCampaignReference"},
}

func validateRequiredFields(obj *unstructured.Unstructured, fieldsList [][]string) error {
	for _, fields := range fieldsList {
		field, found, err := unstructured.NestedMap(obj.Object, fields...)
		if err != nil {
			return fmt.Errorf("Error checking for required field %v: %w", field, err)
		}
		if !found {
			return fmt.Errorf("Required field %v not found", field)
		}
	}
	return nil
}

func validateRequiredSlices(obj *unstructured.Unstructured, fieldsList [][]string) error {
	for _, fields := range fieldsList {
		field, found, err := unstructured.NestedSlice(obj.Object, fields...)
		if err != nil {
			return fmt.Errorf("Error checking for required field %v: %w", field, err)
		}
		if !found {
			return fmt.Errorf("Required field %v not found", field)
		}
	}
	return nil
}

// validateAppProtectPolicy validates Policy resource
func validateAppProtectPolicy(policy *unstructured.Unstructured) error {
	polName := policy.GetName()

	err := validateRequiredFields(policy, appProtectPolicyRequiredFields)
	if err != nil {
		return fmt.Errorf("Error validating App Protect Policy %v: %w", polName, err)
	}

	extRefs, err := checkForExtRefs(policy)
	if err != nil {
		return fmt.Errorf("Error validating App Protect Policy %v: %w", polName, err)
	}

	if len(extRefs) > 0 {
		for _, ref := range extRefs {	
			glog.V(2).Infof("Warning: Field %s (External reference) is Deprecated.", ref)
		}
	}

	return nil
}

// validateAppProtectLogConf validates LogConfiguration resource
func validateAppProtectLogConf(logConf *unstructured.Unstructured) error {
	lcName := logConf.GetName()
	err := validateRequiredFields(logConf, appProtectLogConfRequiredFields)
	if err != nil {
		return fmt.Errorf("Error validating App Protect Log Configuration %v: %w", lcName, err)
	}

	return nil
}

var (
	logDstEx     = regexp.MustCompile(`(?:syslog:server=((?:\d{1,3}\.){3}\d{1,3}|localhost|[a-zA-Z0-9._-]+):\d{1,5})|stderr|(?:\/[\S]+)+`)
	logDstFileEx = regexp.MustCompile(`(?:\/[\S]+)+`)
	logDstFQDNEx = regexp.MustCompile(`(?:[a-zA-Z0-9_-]+\.)+[a-zA-Z0-9_-]+`)
)

// ValidateAppProtectLogDestination validates destination for log configuration
func ValidateAppProtectLogDestination(dstAntn string) error {
	errormsg := "Error parsing App Protect Log config: Destination must follow format: syslog:server=<ip-address | localhost>:<port> or fqdn or stderr or absolute path to file"
	if !logDstEx.MatchString(dstAntn) {
		return fmt.Errorf("%s Log Destination did not follow format", errormsg)
	}
	if dstAntn == "stderr" {
		return nil
	}

	if logDstFileEx.MatchString(dstAntn) {
		return nil
	}

	dstchunks := strings.Split(dstAntn, ":")

	// This error can be ignored since the regex check ensures this string will be parsable
	port, _ := strconv.Atoi(dstchunks[2])

	if port > 65535 || port < 1 {
		return fmt.Errorf("Error parsing port: %v not a valid port number", port)
	}

	ipstr := strings.Split(dstchunks[1], "=")[1]
	if ipstr == "localhost" {
		return nil
	}

	if logDstFQDNEx.MatchString(ipstr) {
		return nil
	}

	if net.ParseIP(ipstr) == nil {
		return fmt.Errorf("Error parsing host: %v is not a valid ip address or host name", ipstr)
	}

	return nil
}

// ParseResourceReferenceAnnotation returns a namespace/name string
func ParseResourceReferenceAnnotation(ns, antn string) string {
	if !strings.Contains(antn, "/") {
		return ns + "/" + antn
	}
	return antn
}

// ParseResourceReferenceAnnotationList returns a slice of ns/names strings
func ParseResourceReferenceAnnotationList(ns, annotations string) []string {
	var out []string
	for _, antn := range strings.Split(annotations, ",") {
		out = append(out, ParseResourceReferenceAnnotation(ns, antn))
	}
	return out
}

func validateAppProtectUserSig(userSig *unstructured.Unstructured) error {
	sigName := userSig.GetName()
	err := validateRequiredSlices(userSig, appProtectUserSigRequiredSlices)
	if err != nil {
		return fmt.Errorf("Error validating App Protect User Signature %v: %w", sigName, err)
	}

	return nil
}

// GetNsName gets the key of a resource in the format: "resNamespace/resName"
func GetNsName(obj *unstructured.Unstructured) string {
	return obj.GetNamespace() + "/" + obj.GetName()
}

func checkForExtRefs(policy *unstructured.Unstructured) ([]string, error) {
	polName := policy.GetName()
	out := []string{}
	for _, ref := range  appProtectPolicyExtRefs {
		_, found, err := unstructured.NestedFieldNoCopy(policy.Object, ref...)
		if err != nil {
			return out, fmt.Errorf("Error validating App Protect Policy %v: %w", polName, err)
		}
		if found {
			out = append(out, strings.Join(ref, "."))	
		}
	}
	return out, nil
}