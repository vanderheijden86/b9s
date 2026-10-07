#!/bin/sh
# Rebuilds examples/sample-project/.beads/issues.jsonl from nothing, with the
# bd on PATH, in an embedded project under a temporary directory. The issues
# are synthetic: a small web shop whose epics hold features, the features hold
# tasks and bugs, and two releases ship the epics.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# Blank every Beads and Dolt setting of the calling shell, so no server
# configuration (a direnv .envrc, a shell profile) can point these writes at a
# real database. awk rather than sed: BSD sed has no alternation in a basic
# regular expression, and a pattern that silently matches nothing unsets
# nothing.
for name in $(env | awk -F= '/^(BEADS|BD|DOLT)_/ { print $1 }'); do
	unset "$name"
done
export BEADS_DIR="$work/.beads" BEADS_DOLT_AUTO_START=0 BD_NON_INTERACTIVE=1 BEADS_ACTOR=sam
# Due and defer dates are calendar days; read them in UTC so the file does not
# depend on the timezone of whoever regenerates it.
export TZ=UTC

# bd records the git identity as each issue's owner. Give it a made-up one
# rather than the identity of whoever runs this script.
printf '[user]\n\tname = sam\n\temail = sam@example.com\n' >"$work/gitconfig"
export GIT_CONFIG_GLOBAL="$work/gitconfig" GIT_CONFIG_NOSYSTEM=1

cd "$work"
bd init --prefix shop --quiet --skip-hooks >/dev/null
# Refuse to write anything unless the project is embedded, not a server.
grep -q '"dolt_mode": "embedded"' .beads/metadata.json

# bd prints a hint about beads.role on stderr for every command.
new() { bd create --silent "$@" 2>/dev/null; }
# bd closes an assigned issue only for its assignee.
close_as() { who=$1; shift; BEADS_ACTOR=$who bd close "$@" >/dev/null 2>&1; }
say() { who=$1; id=$2; shift 2; BEADS_ACTOR=$who bd comments add "$id" -- "$*" >/dev/null 2>&1; }
dep() { bd dep add "$1" "$2" >/dev/null 2>&1; }
status() { bd update "$1" --status="$2" >/dev/null; }

# ------------------------------------------------ Checkout

checkout=$(new --type=epic --priority=1 --title="Checkout" --assignee=sam --labels=checkout \
	--description="Let a visitor pay for the cart in one page, by card or wallet, and get a receipt.")

cart=$(new --type=feature --priority=1 --parent="$checkout" --assignee=sam --title="Shopping cart" --labels=frontend)
cart_totals=$(new --type=task --priority=1 --parent="$cart" --assignee=sam --title="Cart summary with totals and VAT" \
	--labels=frontend --acceptance="Totals match the invoice to the cent.")
cart_keep=$(new --type=task --priority=2 --parent="$cart" --assignee=sam --title="Keep the cart between visits" --labels=frontend,backend)
cart_coupon=$(new --type=task --priority=3 --parent="$cart" --title="Apply a discount code" --labels=frontend)

pay=$(new --type=feature --priority=1 --parent="$checkout" --assignee=alex --title="Card payments through the provider" \
	--labels=backend,payments --design="Redirect to the provider's hosted page. No card data touches our servers.")
pay_redirect=$(new --type=task --priority=1 --parent="$pay" --assignee=alex --title="Redirect to the provider's payment page" --labels=backend,payments)
pay_webhook=$(new --type=task --priority=1 --parent="$pay" --assignee=alex --title="Handle the payment webhook" --labels=backend,payments \
	--description="The provider calls us when a payment settles. Verify the signature, then mark the order paid once, even when the provider retries.

\`\`\`mermaid
sequenceDiagram
    participant Provider
    participant Webhook
    participant Orders
    Provider->>Webhook: payment.settled
    Webhook->>Webhook: verify signature
    Webhook->>Orders: mark paid (idempotent)
    Orders-->>Webhook: ok
    Webhook-->>Provider: 200
\`\`\`")
pay_refund=$(new --type=task --priority=2 --parent="$pay" --title="Refund an order from the admin page" --labels=backend,payments)
pay_double=$(new --type=bug --priority=0 --parent="$pay" --assignee=sam --title="Double click on Pay charges twice" \
	--labels=frontend,payments --description="Two quick clicks send two payment requests. Seen twice in the test shop.")

confirm=$(new --type=feature --priority=2 --parent="$checkout" --title="Order confirmation" --labels=backend)
confirm_page=$(new --type=task --priority=2 --parent="$confirm" --assignee=jo --title="Order confirmation page" --labels=frontend)
confirm_mail=$(new --type=task --priority=2 --parent="$confirm" --title="Email a receipt after payment" --labels=backend)

# ------------------------------------------------ Product search

search=$(new --type=epic --priority=2 --title="Product search" --assignee=robin --labels=search \
	--description="Find a product by name, brand or colour in under a second.")

index=$(new --type=feature --priority=2 --parent="$search" --assignee=robin --title="Search index" --labels=backend)
index_nightly=$(new --type=task --priority=2 --parent="$index" --assignee=robin --title="Build the search index nightly" --labels=backend)
index_live=$(new --type=task --priority=2 --parent="$index" --assignee=robin --title="Update the index when a product changes" --labels=backend)
index_slow=$(new --type=bug --priority=1 --parent="$index" --assignee=robin --title="Search takes 4 s on a cold cache" --labels=backend,performance)

typo=$(new --type=feature --priority=2 --parent="$search" --assignee=robin --title="Match misspelt product names" --labels=backend)
typo_synonyms=$(new --type=task --priority=2 --parent="$typo" --assignee=robin --title="Load a synonym list" --labels=backend)
typo_fuzzy=$(new --type=task --priority=2 --parent="$typo" --title="Fuzzy match on product titles" --labels=backend)

facets=$(new --type=feature --priority=3 --parent="$search" --title="Filter results by brand and colour" --labels=frontend)
facets_counts=$(new --type=task --priority=3 --parent="$facets" --title="Count products per brand and colour in the index" --labels=backend)
facets_panel=$(new --type=task --priority=3 --parent="$facets" --assignee=jo --title="Filter panel on the results page" --labels=frontend)

# ------------------------------------------------ Spring launch

launch=$(new --type=epic --priority=0 --title="Spring launch" --assignee=kim --labels=marketing \
	--description="Everything the spring catalogue needs before it goes live on 15 March.")

catalogue=$(new --type=feature --priority=1 --parent="$launch" --assignee=kim --title="Spring catalogue" --labels=content)
cat_photos=$(new --type=task --priority=1 --parent="$catalogue" --assignee=kim --title="Upload the spring catalogue photos" --labels=content)
cat_prices=$(new --type=task --priority=1 --parent="$catalogue" --assignee=kim --title="Load the spring price list" --labels=content)
cat_texts=$(new --type=task --priority=2 --parent="$catalogue" --title="Write the product descriptions" --labels=content)

campaign=$(new --type=feature --priority=2 --parent="$launch" --assignee=kim --title="Launch campaign" --labels=marketing)
camp_banner=$(new --type=task --priority=2 --parent="$campaign" --title="Homepage banner for spring" --labels=frontend,content)
camp_mail=$(new --type=task --priority=2 --parent="$campaign" --assignee=kim --title="Newsletter to existing customers" --labels=marketing)

loadtest=$(new --type=task --priority=1 --parent="$launch" --assignee=alex --title="Load test checkout at 10x traffic" --labels=performance)

# ------------------------------------------------ Mobile shop

mobile=$(new --type=epic --priority=2 --title="Mobile shop" --assignee=jo --labels=mobile \
	--description="Half the visitors use a phone. Every page works one-handed on a small screen.")

pages=$(new --type=feature --priority=2 --parent="$mobile" --assignee=jo --title="Product pages for small screens" --labels=frontend)
pages_gallery=$(new --type=task --priority=2 --parent="$pages" --assignee=jo --title="Swipe through product photos" --labels=frontend)
pages_button=$(new --type=bug --priority=1 --parent="$pages" --assignee=jo --title="Add to cart button hidden below the fold" --labels=frontend)

wallet=$(new --type=feature --priority=2 --parent="$mobile" --title="Pay with Apple Pay and Google Pay" --labels=frontend,payments)
wallet_button=$(new --type=task --priority=2 --parent="$wallet" --title="Wallet button on the checkout page" --labels=frontend,payments)
wallet_domain=$(new --type=task --priority=3 --parent="$wallet" --assignee=alex --title="Verify the shop domain with Apple" --labels=backend)

# ------------------------------------------------ Seller onboarding

onboarding=$(new --type=epic --priority=3 --title="Seller onboarding" --labels=sellers \
	--description="Sellers list their first product without help.")

signup=$(new --type=feature --priority=3 --parent="$onboarding" --title="Seller sign-up" --labels=backend)
signup_form=$(new --type=task --priority=3 --parent="$signup" --title="Seller sign-up form" --labels=frontend)
signup_bank=$(new --type=task --priority=3 --parent="$signup" --title="Verify the seller's bank account" --labels=backend,payments)

dashboard=$(new --type=feature --priority=3 --parent="$onboarding" --title="Seller dashboard" --labels=frontend)
dash_products=$(new --type=task --priority=3 --parent="$dashboard" --title="List and edit products" --labels=frontend)
dash_sales=$(new --type=task --priority=4 --parent="$dashboard" --title="Sales per week" --labels=frontend)

guide=$(new --type=task --priority=4 --parent="$onboarding" --title="Write the seller guide" --labels=content)

# ------------------------------------------------ outside the epics

logo=$(new --type=chore --priority=4 --title="Replace the favicon with the new logo" --labels=frontend)
deps=$(new --type=chore --priority=3 --assignee=alex --title="Update the web framework to the next minor release")

# ------------------------------------------------ releases (milestones)

spring=$(new --type=milestone --priority=0 --due=2027-03-15 --assignee=kim --title="Spring release" \
	--description="The shop takes payments and shows the spring catalogue.")
marketplace=$(new --type=milestone --priority=2 --due=2027-06-30 --assignee=sam --title="Marketplace release" \
	--description="Other sellers list and sell their own products.")

# A release depends on each epic it ships (ADR 0026). Product search and the
# mobile shop have no release yet, so the release views list them as
# unscheduled.
dep "$spring" "$checkout"
dep "$spring" "$launch"
dep "$marketplace" "$onboarding"

# Blocking work: the arrows the dependency graph and the blocked filter show.
dep "$launch" "$checkout"
dep "$pay_webhook" "$pay_redirect"
dep "$confirm_mail" "$pay_webhook"
dep "$pay_refund" "$pay_webhook"
dep "$loadtest" "$pay"
dep "$camp_banner" "$cat_photos"
dep "$camp_mail" "$cat_texts"
dep "$typo_fuzzy" "$index_nightly"
dep "$facets_counts" "$index_nightly"
dep "$facets_panel" "$facets_counts"
dep "$wallet" "$pay"
dep "$signup_bank" "$signup_form"
dep "$dash_products" "$signup_form"

status "$pay" in_progress
status "$pay_webhook" in_progress
status "$pay_double" in_progress
status "$cart_keep" in_progress
status "$index_slow" in_progress
status "$typo_synonyms" in_progress
status "$catalogue" in_progress
status "$cat_photos" in_progress
status "$pages_button" in_progress
bd update "$loadtest" --status=blocked --notes="Waits for the payment provider's sandbox keys." >/dev/null
bd defer "$guide" --until=2027-03-01 >/dev/null
bd defer "$wallet_domain" --until=2027-01-15 >/dev/null

say sam "$pay_double" "Reproduced on the staging shop with a slow network."
say jo "$pay_double" "Disabling the button after the first click fixes it locally."
say alex "$pay" "Provider sandbox account requested."
say alex "$pay_webhook" "The provider retries for 24 hours, so the handler must accept the same event twice."
say robin "$index_slow" "Most of the time goes to loading the synonym list."
say kim "$cat_photos" "120 of 180 photos are up. The rest arrive from the studio on Friday."
say jo "$pages_button" "Only on screens shorter than 640 px."

close_as sam "$cart_totals" --reason="Shipped"
close_as alex "$pay_redirect" --reason="Live in the test shop"
close_as robin "$index_nightly" --reason="Runs at 02:00"
close_as kim "$cat_prices" --reason="Loaded"
close_as sam "$logo" --reason="Done"
close_as sam "$signup_form" --reason="Built"

mkdir -p "$here/.beads"
bd export >"$here/.beads/issues.jsonl"
echo "wrote $(wc -l <"$here/.beads/issues.jsonl" | tr -d ' ') issues to $here/.beads/issues.jsonl"
