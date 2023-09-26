package webhooks

import (
	"github.com/davidmogar/release-service/api/v1alpha1/webhooks/author"
	"github.com/davidmogar/release-service/api/v1alpha1/webhooks/release"
	"github.com/davidmogar/release-service/api/v1alpha1/webhooks/releaseplan"
	"github.com/davidmogar/release-service/api/v1alpha1/webhooks/releaseplanadmission"
	"github.com/redhat-appstudio/operator-toolkit/webhook"
)

// EnabledWebhooks is a slice containing references to all the webhooks that have to be registered
var EnabledWebhooks = []webhook.Webhook{
	&author.Webhook{},
	&release.Webhook{},
	&releaseplan.Webhook{},
	&releaseplanadmission.Webhook{},
}
