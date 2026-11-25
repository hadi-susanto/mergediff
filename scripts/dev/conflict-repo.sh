#!/usr/bin/env bash
# Creates a Git repository stopped in the middle of a merge with the
# conflicts of the Conflicts window mockups (docs/mockups/mergediff-dir-merge-*):
# merging origin/main into feature/checkout leaves
#
#   text conflicts (both modified)   StripePaymentGatewayAdapter.java,
#                                    PaymentGatewayRegistry.java,
#                                    CheckoutSummaryPanel.tsx
#   both added                       V42__add_checkout_session_table.sql
#   modify/delete (theirs deleted)   LegacyCartCalculator.java
#   modify/delete (yours deleted)    LegacyCartCalculatorTest.java
#   binary                           checkout-banner@2x.png
#   both modified (small)            pom.xml, config/application-production.yml
#
# The mockups show the last two resolved: accept them in the window.
# Usage: conflict-repo.sh DIR   (DIR must not exist or be empty)
# Then:  cd DIR && mergediff resolve      (or: make dev-resolve)
set -eu

dir=${1:?usage: conflict-repo.sh DIR}
if [ -e "$dir" ] && [ -n "$(ls -A "$dir" 2>/dev/null)" ]; then
  echo "conflict-repo.sh: $dir is not empty" >&2
  exit 2
fi
mkdir -p "$dir"
cd "$dir"

# Ignore the user's Git configuration (signing, hooks, autocrlf).
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=mergediff GIT_AUTHOR_EMAIL=dev@example.invalid
export GIT_COMMITTER_NAME=mergediff GIT_COMMITTER_EMAIL=dev@example.invalid
git init -q
git symbolic-ref HEAD refs/heads/main

PAY=src/main/java/com/example/shop/payment/gateway/internal
LEGACY=src/main/java/com/example/shop/checkout/legacy
LEGACY_TEST=src/test/java/com/example/shop/checkout/legacy
PANEL=frontend/src/features/checkout/components/CheckoutSummaryPanel.tsx
BANNER=frontend/public/images/checkout-banner@2x.png
SQL=src/main/resources/db/migration/V42__add_checkout_session_table.sql
mkdir -p "$PAY" "$LEGACY" "$LEGACY_TEST" "$(dirname "$PANEL")" "$(dirname "$BANNER")" "$(dirname "$SQL")" config

# adapter TIMEOUT RETRIES CURRENCY LOG: the adapter with four values that
# the branches change (some on both sides: conflicts; some on one side).
adapter() {
  cat <<EOF
package com.example.shop.payment.gateway.internal;

import java.time.Duration;

/** Talks to Stripe for the checkout. */
public final class StripePaymentGatewayAdapter implements PaymentGateway {
    private static final Duration TIMEOUT = Duration.ofSeconds($1);
    private static final int MAX_RETRIES = $2;

    private final StripeClient client;

    public StripePaymentGatewayAdapter(StripeClient client) {
        this.client = client;
    }

    @Override
    public PaymentResult charge(Order order) {
        String currency = "$3";
        log.info("$4", order.id());
        return client.charge(order.total(), currency, TIMEOUT, MAX_RETRIES);
    }

    @Override
    public void refund(Payment payment) {
        client.refund(payment.id());
    }

    @Override
    public String name() {
        return "stripe";
    }
}
EOF
}

registry() {
  cat <<EOF
package com.example.shop.payment.gateway.internal;

import java.util.Map;

public final class PaymentGatewayRegistry {
    private final Map<String, PaymentGateway> gateways;

    public PaymentGatewayRegistry(Map<String, PaymentGateway> gateways) {
        this.gateways = $1;
    }

    public PaymentGateway get(String name) {
        return $2;
    }
}
EOF
}

panel() {
  cat <<EOF
export function CheckoutSummaryPanel({ order }: Props) {
  return (
    <section className="$1">
      <h2>$2</h2>
      <Total value={order.total} />
    </section>
  );
}
EOF
}

calculator() {
  cat <<EOF
package com.example.shop.checkout.legacy;

/** Old cart totals, kept for the mobile app. */
public final class LegacyCartCalculator {
    public long total(Cart cart) {
        return cart.items().stream().mapToLong(Item::price).sum()$1;
    }
}
EOF
}

calculator_test() {
  cat <<EOF
package com.example.shop.checkout.legacy;

class LegacyCartCalculatorTest {
    @Test
    void sumsItems() {
        assertEquals($1, new LegacyCartCalculator().total(cart(10, 20)));
    }
}
EOF
}

pom() {
  printf '<project>\n  <artifactId>shop-api</artifactId>\n  <version>%s</version>\n</project>\n' "$1"
}

yml() {
  printf 'payment:\n  provider: stripe\n  timeout: %s\n' "$1"
}

banner() {
  printf '\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR%s' "$1"
}

# Base commit (main).
adapter 10 3 USD "charging order {}" >"$PAY/StripePaymentGatewayAdapter.java"
registry gateways 'gateways.get(name)' >"$PAY/PaymentGatewayRegistry.java"
panel summary "Summary" >"$PANEL"
calculator "" >"$LEGACY/LegacyCartCalculator.java"
calculator_test 30 >"$LEGACY_TEST/LegacyCartCalculatorTest.java"
pom 1.0.0 >pom.xml
yml 10s >config/application-production.yml
banner base >"$BANNER"
printf '# shop-api\n' >README.md
git add -A
git commit -q -m "Initial shop-api"

# origin/main: upstream changes.
git checkout -q -b upstream
adapter 15 5 EUR "charging order {}" >"$PAY/StripePaymentGatewayAdapter.java"
registry 'Map.copyOf(gateways)' 'gateways.get(name)' >"$PAY/PaymentGatewayRegistry.java"
panel "summary summary--compact" "Order summary" >"$PANEL"
git rm -q "$LEGACY/LegacyCartCalculator.java"
calculator_test 31 >"$LEGACY_TEST/LegacyCartCalculatorTest.java"
printf 'CREATE TABLE checkout_session (id UUID PRIMARY KEY);\n' >"$SQL"
pom 1.1.0 >pom.xml
yml 15s >config/application-production.yml
banner upstream >"$BANNER"
git add -A
git commit -q -m "Upstream: EUR, compact summary, drop legacy calculator"

# feature/checkout: your changes.
git checkout -q -b feature/checkout main
adapter 20 3 GBP "charging order {} with stripe" >"$PAY/StripePaymentGatewayAdapter.java"
registry 'new HashMap<>(gateways)' 'gateways.getOrDefault(name, fallback)' >"$PAY/PaymentGatewayRegistry.java"
panel "checkout-summary" "Your order" >"$PANEL"
calculator " + cart.shipping()" >"$LEGACY/LegacyCartCalculator.java"
git rm -q "$LEGACY_TEST/LegacyCartCalculatorTest.java"
mkdir -p "$(dirname "$SQL")" # removed with upstream's file when switching branches
printf 'CREATE TABLE checkout_session (id BIGSERIAL PRIMARY KEY, created_at TIMESTAMP);\n' >"$SQL"
pom 1.0.1 >pom.xml
yml 20s >config/application-production.yml
banner feature >"$BANNER"
git add -A
git commit -q -m "Checkout: GBP, shipping in legacy totals"

# Publish upstream as origin/main only, so the labels read like the mockup.
git update-ref refs/remotes/origin/main upstream
git branch -q -D upstream main

if git merge -q --no-edit origin/main >/dev/null 2>&1; then
  echo "conflict-repo.sh: the merge did not conflict" >&2
  exit 1
fi
echo "conflict-repo.sh: $(git diff --name-only --diff-filter=U | wc -l | tr -d ' ') conflicts in $(pwd)"
