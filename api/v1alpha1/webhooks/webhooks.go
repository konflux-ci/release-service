package webhooks

import (
	"github.com/konflux-ci/operator-toolkit/webhook"
	"github.com/konflux-ci/release-service/api/v1alpha1/webhooks/author"
	"github.com/konflux-ci/release-service/api/v1alpha1/webhooks/release"
	"github.com/konflux-ci/release-service/api/v1alpha1/webhooks/releaseplan"
	"github.com/konflux-ci/release-service/api/v1alpha1/webhooks/releaseplanadmission"
)

// EnabledWebhooks is a slice containing references to all the webhooks that have to be registered
var EnabledWebhooks = []webhook.Webhook{
	&author.Webhook{},
	&release.Webhook{},
	&releaseplan.Webhook{},
	&releaseplanadmission.Webhook{},
}
