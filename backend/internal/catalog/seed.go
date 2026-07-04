package catalog

import "github.com/yesoreyeram/data-explorer/backend/internal/domain"

// seed is a hand-curated starting point, not an attempt at an exhaustive
// registry - it exists to save a few minutes of "what's the base URL and
// auth scheme for X" lookup, not to guarantee correctness for every
// deployment. Base URLs containing a `{placeholder}` (a per-tenant subdomain
// or workspace id) still need editing before the connection will work; the
// form is prefilled, not final.
var seed = []Entry{
	{
		ID: "github-rest", Name: "GitHub", Description: "GitHub's REST API - repos, issues, actions, and more.",
		Category: "Developer tools", Type: domain.ConnectionTypeREST, BaseURL: "https://api.github.com",
		AuthType: "bearer", DocsURL: "https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api",
	},
	{
		ID: "github-graphql", Name: "GitHub GraphQL", Description: "GitHub's GraphQL API v4.",
		Category: "Developer tools", Type: domain.ConnectionTypeGraphQL, Endpoint: "https://api.github.com/graphql",
		AuthType: "bearer", DocsURL: "https://docs.github.com/en/graphql/guides/forming-calls-with-graphql",
	},
	{
		ID: "stripe", Name: "Stripe", Description: "Payments, subscriptions, and billing.",
		Category: "Payments", Type: domain.ConnectionTypeREST, BaseURL: "https://api.stripe.com/v1",
		AuthType: "basic", DocsURL: "https://stripe.com/docs/api/authentication",
	},
	{
		ID: "slack", Name: "Slack", Description: "Slack's Web API - messages, channels, users.",
		Category: "Messaging", Type: domain.ConnectionTypeREST, BaseURL: "https://slack.com/api",
		AuthType: "bearer", DocsURL: "https://api.slack.com/authentication/basics",
	},
	{
		ID: "twilio", Name: "Twilio", Description: "SMS, voice, and messaging APIs.",
		Category: "Communications", Type: domain.ConnectionTypeREST, BaseURL: "https://api.twilio.com/2010-04-01",
		AuthType: "basic", DocsURL: "https://www.twilio.com/docs/iam/api",
	},
	{
		ID: "sendgrid", Name: "SendGrid", Description: "Transactional and marketing email.",
		Category: "Email", Type: domain.ConnectionTypeREST, BaseURL: "https://api.sendgrid.com/v3",
		AuthType: "bearer", DocsURL: "https://docs.sendgrid.com/for-developers/sending-email/authentication",
	},
	{
		ID: "hubspot", Name: "HubSpot", Description: "CRM, marketing, and sales objects.",
		Category: "CRM", Type: domain.ConnectionTypeREST, BaseURL: "https://api.hubapi.com",
		AuthType: "bearer", DocsURL: "https://developers.hubspot.com/docs/api/private-apps",
	},
	{
		ID: "shopify-rest", Name: "Shopify Admin (REST)", Description: "Store, products, and orders - REST Admin API.",
		Category: "Commerce", Type: domain.ConnectionTypeREST, BaseURL: "https://{shop}.myshopify.com/admin/api/2024-01",
		AuthType: "apiKey", AuthConfig: map[string]any{"apiKeyHeader": "X-Shopify-Access-Token"},
		DocsURL: "https://shopify.dev/docs/api/admin-rest",
	},
	{
		ID: "shopify-graphql", Name: "Shopify Admin (GraphQL)", Description: "Store, products, and orders - GraphQL Admin API.",
		Category: "Commerce", Type: domain.ConnectionTypeGraphQL, Endpoint: "https://{shop}.myshopify.com/admin/api/2024-01/graphql.json",
		AuthType: "apiKey", AuthConfig: map[string]any{"apiKeyHeader": "X-Shopify-Access-Token"},
		DocsURL: "https://shopify.dev/docs/api/admin-graphql",
	},
	{
		ID: "notion", Name: "Notion", Description: "Pages, databases, and blocks.",
		Category: "Productivity", Type: domain.ConnectionTypeREST, BaseURL: "https://api.notion.com/v1",
		AuthType: "bearer", DocsURL: "https://developers.notion.com/docs/authorization",
	},
	{
		ID: "airtable", Name: "Airtable", Description: "Bases, tables, and records.",
		Category: "Productivity", Type: domain.ConnectionTypeREST, BaseURL: "https://api.airtable.com/v0",
		AuthType: "bearer", DocsURL: "https://airtable.com/developers/web/api/authentication",
	},
	{
		ID: "salesforce", Name: "Salesforce", Description: "CRM objects via the REST API.",
		Category: "CRM", Type: domain.ConnectionTypeREST, BaseURL: "https://{instance}.salesforce.com/services/data/v60.0",
		AuthType: "oauth2RefreshToken", AuthConfig: map[string]any{"oauth2TokenUrl": "https://login.salesforce.com/services/oauth2/token"},
		DocsURL: "https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/intro_understanding_authentication.htm",
	},
	{
		ID: "zendesk", Name: "Zendesk", Description: "Tickets, users, and organizations.",
		Category: "Customer support", Type: domain.ConnectionTypeREST, BaseURL: "https://{subdomain}.zendesk.com/api/v2",
		AuthType: "basic", DocsURL: "https://developer.zendesk.com/documentation/ticketing/security-and-auth/making-basic-authenticated-requests-to-zendesk-apis/",
	},
	{
		ID: "pagerduty", Name: "PagerDuty", Description: "Incidents, services, and on-call schedules.",
		Category: "Incident management", Type: domain.ConnectionTypeREST, BaseURL: "https://api.pagerduty.com",
		AuthType: "apiKey", AuthConfig: map[string]any{"apiKeyHeader": "Authorization"},
		DocsURL: "https://developer.pagerduty.com/docs/rest-api-v2/authentication/",
	},
	{
		ID: "linear", Name: "Linear", Description: "Issues, projects, and cycles.",
		Category: "Project management", Type: domain.ConnectionTypeGraphQL, Endpoint: "https://api.linear.app/graphql",
		AuthType: "apiKey", AuthConfig: map[string]any{"apiKeyHeader": "Authorization"},
		DocsURL: "https://developers.linear.app/docs/graphql/working-with-the-graphql-api#personal-api-keys",
	},
	{
		ID: "contentful", Name: "Contentful", Description: "Content Delivery API over GraphQL.",
		Category: "CMS", Type: domain.ConnectionTypeGraphQL, Endpoint: "https://graphql.contentful.com/content/v1/spaces/{space_id}",
		AuthType: "bearer", DocsURL: "https://www.contentful.com/developers/docs/references/graphql/",
	},
	{
		ID: "hasura", Name: "Hasura", Description: "Auto-generated GraphQL over your database.",
		Category: "Database/API", Type: domain.ConnectionTypeGraphQL, Endpoint: "https://{your-app}.hasura.app/v1/graphql",
		AuthType: "apiKey", AuthConfig: map[string]any{"apiKeyHeader": "x-hasura-admin-secret"},
		DocsURL: "https://hasura.io/docs/latest/auth/authentication/admin-secret-header/",
	},
	{
		ID: "discord", Name: "Discord", Description: "Bot API - guilds, channels, and messages.",
		Category: "Social", Type: domain.ConnectionTypeREST, BaseURL: "https://discord.com/api/v10",
		AuthType: "apiKey", AuthConfig: map[string]any{"apiKeyHeader": "Authorization"},
		DocsURL: "https://discord.com/developers/docs/reference#authentication",
	},
	{
		ID: "openai", Name: "OpenAI", Description: "Chat, completions, and embeddings.",
		Category: "AI", Type: domain.ConnectionTypeREST, BaseURL: "https://api.openai.com/v1",
		AuthType: "bearer", DocsURL: "https://platform.openai.com/docs/api-reference/authentication",
	},
	{
		ID: "mailgun", Name: "Mailgun", Description: "Transactional email sending and validation.",
		Category: "Email", Type: domain.ConnectionTypeREST, BaseURL: "https://api.mailgun.net/v3",
		AuthType: "basic", DocsURL: "https://documentation.mailgun.com/docs/mailgun/api-reference/#authentication",
	},
}
