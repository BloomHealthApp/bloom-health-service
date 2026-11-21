package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
)

// AndroidPackage is the app's Android package name, which App Links verify against.
const AndroidPackage = "com.bloomhealthapp.app"

type assetTarget struct {
	Namespace              string   `json:"namespace"`
	PackageName            string   `json:"package_name"`
	SHA256CertFingerprints []string `json:"sha256_cert_fingerprints"`
}

type assetStatement struct {
	Relation []string    `json:"relation"`
	Target   assetTarget `json:"target"`
}

// HandleGetAssetLinks is GET /.well-known/assetlinks.json, which lets Android open /u/ links in the app. With no
// fingerprints configured it serves an empty list, so links open the web page instead.
func HandleGetAssetLinks(c *gin.Context) {
	fps := di.AppLogic.GetConfig().AndroidCertSHA256s
	out := []assetStatement{}
	if len(fps) > 0 {
		out = append(out, assetStatement{Relation: []string{"delegate_permission/common.handle_all_urls"},
			Target: assetTarget{Namespace: "android_app", PackageName: AndroidPackage, SHA256CertFingerprints: fps}})
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, out)
}
