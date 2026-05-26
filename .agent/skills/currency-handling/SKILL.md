---
name: currency-handling
description: "Best practices for currency handling, currency conversions, billing ledger schemas, exchange rate integration, and cryptocurrency precision. Use this skill when managing currency operations, tracking monetary transactions, designing financial ledger schemas, or integrating with exchange rate APIs."
---

# Currency Handling Best Practices

When building systems that process monetary transactions, invoice users, handle billing, or maintain financial ledger state across multiple currencies, you must follow strict invariants to ensure auditability, consistency, and correctness.

**Persona:** You are a financial systems architect. You treat money with absolute precision — every transaction must be immutable, every conversion traceable, and floating-point errors are considered an existential threat.

**Modes:**
- **Design mode** — designing database schemas, transaction flows, and APIs for multi-currency applications.
- **Review/Audit mode** — reviewing existing billing, ledger, or conversion code to ensure compliance with financial consistency and precision invariants.

---

## Architectural Invariants and Anti-Patterns

### 1. Dynamic Conversion at Query/View Time is Forbidden
Exchange rates are constantly fluctuating. Calculating the converted value of historical transactions on the fly using the *current* exchange rate is a severe anti-pattern.
* **Why:** If a user is billed 10 USD on Monday (equivalent to 9 EUR at that time), showing that transaction as 9.5 EUR next month because the rate shifted breaks reconciliation, invoice accuracy, and legal tax reporting.
* **Invariant:** Every transaction record must be **immutable**. You must capture and store the settled values and exchange rates permanently at the exact moment the transaction is executed or when the quote is locked.

### 2. Floating-Point Types are Forbidden for Monetary Calculations
Never use floating-point types (such as float/double) for monetary amounts or exchange rates.
* **Why:** Floats exhibit binary rounding errors (e.g., `0.1 + 0.2` does not equal `0.3`), which accumulate over time and cause balance mismatches in ledgers.
* **Invariant:** You must use either **arbitrary-precision decimals** or **integers representing minor units** (e.g., cents, Satoshis, Wei) for all monetary arithmetic.

---

## Multi-Currency Representation and Precision

Different asset classes and currencies require distinct scale and precision strategies. Your data storage and business logic must handle these differences explicitly.

### Minor Units vs. Arbitrary Precision Decimals
1. **Fiat Currencies:**
   * Most fiat currencies use a scale of 2 (e.g., USD, EUR have 100 cents per unit).
   * Some use a scale of 0 (e.g., JPY, KRW) or 3 (e.g., BHD, KWD, OMR).
   * Store these as integers representing the smallest minor unit, or as fixed-point decimals with the appropriate scale.
2. **Cryptocurrencies:**
   * Cryptocurrencies require much higher precision. For example, Bitcoin uses 8 decimals (Satoshis), and Ethereum uses 18 decimals (Wei).
   * Storing crypto requires arbitrary-precision decimal types or large-scale integer structures to avoid overflow and loss of precision.
3. **Conversion / Exchange Rates:**
   * Exchange rates require higher precision than the currencies themselves. A rate must often be tracked to 6, 8, or more decimal places (e.g., `1 USD = 0.923456 EUR`) to prevent rounding discrepancies in large transactions.

---

## Historical Rate Locking and Quote Management

To ensure that the price presented to a user matches the amount charged, you must decouple the *pricing phase* (quoting) from the *execution phase* (settlement).

### The Quote Lifecycle
1. **Quote Request:** When a user requests a currency conversion or is presented with a multi-currency price, generate a temporary **Quote**.
2. **Quote Invariants:** A Quote must contain:
   * Unique Quote Identifier.
   * Source amount and currency.
   * Target amount and currency.
   * The locked exchange rate applied.
   * The rate provider/source identification.
   * An expiration timestamp (Time-To-Live). Rates should be locked only for a short window (e.g., 5 to 10 minutes) to mitigate exchange rate risk.
3. **Settlement Integration:** When the transaction is completed, the payment/ledger record must reference the **Quote ID**. The settled amount is then recorded using the exact locked exchange rate from that quote, even if the execution occurred hours or days later (provided the quote was valid at the start of execution).

---

## Multi-Provider Rate Sourcing and Routing

Different currency pairs (e.g., USD/EUR vs. BTC/USD) require different exchange rate feeds. A robust multi-currency system must route rate queries and conversion requests appropriately.

### 1. Data Provider Routing
* **Fiat-to-Fiat Routing:** Route queries for official fiat exchange rates to reputable sources, such as Central Banks (e.g., European Central Bank, Federal Reserve) or commercial foreign exchange feeds.
* **Crypto-to-Fiat Routing:** Route cryptocurrency pricing to specific decentralized oracles, centralized exchange APIs, or aggregators that reflect active market order books.
* **Custom/Negotiated Sourcing:** In B2B or specialized transactions, route conversions using custom rate sheets, contracts, or spread rules established for the specific client.

### 2. Fallback and Timeout Strategies
Because exchange rate providers can experience downtime, you must design fallback behaviors:
* **Staleness Limits:** Define a maximum acceptable age for cached rates (e.g., 24 hours for daily Central Bank rates, 1 minute for crypto/volatile markets). If the cache is older than this limit, the rate is "stale" and must not be used.
* **Fallback Cascades:** If the primary rate provider is unreachable or returns an error, cascade to a configured secondary provider.
* **Degraded Operation:** If all providers are offline and cached rates are stale, refuse to execute new dynamic conversions to protect the system from financial risk.

### 3. Spread, Margin, and Fee Accounting
Commercial systems rarely convert currencies at the raw mid-market rate. They apply a markup (spread) for profit or risk mitigation.
* **Invariant:** You must explicitly separate the **raw market rate** from the **applied rate** (market rate + markup) in transaction metadata.
* **Why:** This distinction is necessary to audit revenue generated from currency conversions and to calculate correct spread margins for tax and business analysis.
