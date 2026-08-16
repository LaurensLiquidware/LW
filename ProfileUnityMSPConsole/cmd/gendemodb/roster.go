package main

// story identifies which per-tenant narrative generateHistory follows when
// computing a day's utilization fraction (see generate.go's
// baseUtilization). Each value corresponds to exactly one roster entry
// below, matching the task brief's ten-tenant table.
type story int

const (
	storyGrowth         story = iota // Noordkaap: steady organic growth, 55% -> 68%
	storyAmber                       // Vaandel: approaching the limit, ~94%
	storyOversubscribed              // Bakhuis: crosses 100% in month 5
	storyFlat                        // Zandpoort: flat, low utilization ~40%
	storyRenewalWarning              // Kroon: nothing special about usage; support expires soon
	storyTrial                       // Helix: evaluation tenant, trial expires mid-window
	storySeasonal                    // Delta: term-time peaks, deep summer floor
	storyFlexOnly                    // Meridiaan: FlexApp-only entitlement
	storyLateOnboard                 // Zonneveld: onboarded 6 weeks ago, history starts late
	storyOutage                      // Havenbedrijf: 5-day collection outage mid-period
)

// tenantSpec describes one fictional roster entry and the parameters
// generateHistory needs to render its story as daily snapshots.
type tenantSpec struct {
	displayName    string
	hostname       string // reserved/unroutable (RFC 2606 *.example.com) -- see README's "no outbound calls" rail
	port           int
	mode           string // ProfileUnity LicenseMode/LicenseProduct's "mode": NamedUser, Concurrent, or Evaluation
	product        string
	registeredTo   string // ProfileUnity's own "RegisteredTo" field -- deliberately varied from displayName
	consoleVersion string
	totalLicenses  int // current/final entitlement; see entitlementChangeTenant for the one mid-period change
	story          story

	isTrial    bool
	isFlexOnly bool

	// supportEndsOffsetDays is SupportEnds relative to --end-date -- may be
	// negative (already expired) or small/positive (renewal warning).
	supportEndsOffsetDays int

	// notes seeds the tenant's Notes field with a one-line reminder of
	// which UI state this entry exists to exercise, so the roster doubles
	// as documentation inside the running demo, not just in this file.
	notes string
}

// entitlementChangeTenant is the index into roster whose TotalLicenses
// changes exactly once mid-period (a seat purchase) -- see generate.go's
// totalLicensesOnDay. Noordkaap's steady-growth story is the natural fit:
// growth stories plausibly include a capacity top-up.
const entitlementChangeTenant = 0

// entitlementChangeBeforeTotal is Noordkaap's TotalLicenses before the
// mid-period change; roster[entitlementChangeTenant].totalLicenses is the
// value after.
const entitlementChangeBeforeTotal = 650

var roster = []tenantSpec{
	{
		displayName: "Noordkaap Zorggroep", hostname: "noordkaap.demo.example.com", port: 8000,
		mode: "NamedUser", product: "ProU+FlexApp", registeredTo: "Noordkaap Zorggroep",
		consoleVersion: "6.9.5.9678 3038806 2026-07-01", totalLicenses: 750,
		story: storyGrowth, supportEndsOffsetDays: 220,
		notes: "Demo tenant: healthy, steady organic growth (55% -> 68%); one mid-period seat purchase (650 -> 750).",
	},
	{
		displayName: "Vaandel Logistics", hostname: "vaandel.demo.example.com", port: 8000,
		mode: "Concurrent", product: "ProU+FlexApp", registeredTo: "Vaandel Logistics B.V.",
		consoleVersion: "6.9.4.9530 3021144 2026-04-12", totalLicenses: 250,
		story: storyAmber, supportEndsOffsetDays: 140,
		notes: "Demo tenant: approaching the license limit (~94%) -- amber threshold state.",
	},
	{
		displayName: "Bakhuis Retail Group", hostname: "bakhuis.demo.example.com", port: 8000,
		mode: "NamedUser", product: "ProU+FlexApp", registeredTo: "Bakhuis Retail Group",
		consoleVersion: "6.9.5.9678 3038806 2026-07-01", totalLicenses: 500,
		story: storyOversubscribed, supportEndsOffsetDays: 95,
		notes: "Demo tenant: over-subscribed, crosses 100% of entitlement in month 5 -- red state.",
	},
	{
		displayName: "Gemeente Zandpoort", hostname: "zandpoort.demo.example.com", port: 8443,
		mode: "NamedUser", product: "ProFlex", registeredTo: "Gemeente Zandpoort",
		consoleVersion: "6.9.3.9410 3009231 2026-01-08", totalLicenses: 1200,
		story: storyFlat, supportEndsOffsetDays: 300,
		notes: "Demo tenant: flat, low utilization (~40%) -- the stranded-capacity case.",
	},
	{
		displayName: "Kroon Financieel", hostname: "kroon.demo.example.com", port: 8000,
		mode: "NamedUser", product: "ProU+FlexApp", registeredTo: "Kroön Financiële Diensten",
		consoleVersion: "6.9.5.9678 3038806 2026-07-01", totalLicenses: 150,
		story: storyRenewalWarning, supportEndsOffsetDays: 21,
		notes: "Demo tenant: support contract expires in 21 days -- renewal warning. Non-ASCII RegisteredTo on purpose.",
	},
	{
		displayName: "Helix BioLab", hostname: "helix.demo.example.com", port: 8000,
		mode: "Evaluation", product: "ProU+FlexApp", registeredTo: "Helix BioLab",
		consoleVersion: "6.9.5.9678 3038806 2026-07-01", totalLicenses: 100,
		story: storyTrial, isTrial: true, supportEndsOffsetDays: 10,
		notes: "Demo tenant: trial tenant; IsTrialExpired flips from false to true partway through the window.",
	},
	{
		displayName: "Delta Onderwijs", hostname: "delta-onderwijs.demo.example.com", port: 8000,
		mode: "NamedUser", product: "ProU+FlexApp", registeredTo: "Stichting Delta Onderwijs",
		consoleVersion: "6.9.5.9678 3038806 2026-07-01", totalLicenses: 2000,
		story: storySeasonal, supportEndsOffsetDays: 250,
		notes: "Demo tenant: strong seasonality -- term-time peaks, deep summer floor.",
	},
	{
		displayName: "Meridiaan Advocaten", hostname: "meridiaan.demo.example.com", port: 8000,
		mode: "NamedUser", product: "FlexApp", registeredTo: "Meridiaan Advocaten",
		consoleVersion: "6.9.4.9530 3021144 2026-04-12", totalLicenses: 75,
		story: storyFlexOnly, isFlexOnly: true, supportEndsOffsetDays: 180,
		notes: "Demo tenant: IsFlexOnly -- FlexApp-only entitlement rendering.",
	},
	{
		displayName: "Zonneveld Techniek", hostname: "zonneveld.demo.example.com", port: 8000,
		mode: "NamedUser", product: "ProU+FlexApp", registeredTo: "Zonneveld Techniek",
		consoleVersion: "6.9.5.9678 3038806 2026-07-01", totalLicenses: 300,
		story: storyLateOnboard, supportEndsOffsetDays: 200,
		notes: "Demo tenant: onboarded 6 weeks ago -- history legitimately starts late (absent days, not zeros).",
	},
	{
		displayName: "Havenbedrijf Rijnmond", hostname: "havenbedrijf-rijnmond.demo.example.com", port: 8000,
		mode: "Concurrent", product: "ProU+FlexApp", registeredTo: "Havenbedrijf Rijnmond",
		consoleVersion: "6.8.9.9012 2987710 2025-10-02", totalLicenses: 900,
		story: storyOutage, supportEndsOffsetDays: 165,
		notes: "Demo tenant: 5-day collection outage mid-period, five distinct failure causes; older console version.",
	},
}
