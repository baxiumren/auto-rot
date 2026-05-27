# 🔧 Klikcepat Patch: Enable Shared Domains in /api/domains

## 🎯 Problem

Sub-user akun klikcepat **gak bisa liat shared domain** (domain yang di-share dari master account) via `/api/domains` endpoint, walau dashboard nampilin domain itu. Penyebab: query API filter strict `user_id = sub_user_id`, gak include shared domains dengan `type = 1`.

Akibat: bot BongBot gak bisa auto-detect domain shortlink → display fallback ke klikcepat.com default (salah).

## ✅ Solution

Patch file `ApiDomains.php` supaya pakai query yang sama dengan dashboard's `get_available_domains_by_user()` — include shared domains.

## 📋 File yang Di-patch

**File:** `app/controllers/api/ApiDomains.php`

**Locations:** 3 SQL query yang perlu di-update:

### Patch 1 — Line 77 (get_all: COUNT query)

**Before:**
```php
$total_rows = database()->query("SELECT COUNT(*) AS `total` FROM `domains` WHERE `user_id` = {$this->api_user->user_id}")->fetch_object()->total ?? 0;
```

**After:**
```php
$total_rows = database()->query("SELECT COUNT(*) AS `total` FROM `domains` WHERE (`user_id` = {$this->api_user->user_id} OR `type` = 1) AND `is_enabled` = 1")->fetch_object()->total ?? 0;
```

### Patch 2 — Line ~88 (get_all: SELECT query)

**Before:**
```php
SELECT
    *
FROM
    `domains`
WHERE
    `user_id` = {$this->api_user->user_id}
    {$filters->get_sql_where()}
    {$filters->get_sql_order_by()}
```

**After:**
```php
SELECT
    *
FROM
    `domains`
WHERE
    (`user_id` = {$this->api_user->user_id} OR `type` = 1) AND `is_enabled` = 1
    {$filters->get_sql_where()}
    {$filters->get_sql_order_by()}
```

### Patch 3 — Line ~137 (get single)

**Before:**
```php
$domain = db()->where('domain_id', $domain_id)->where('user_id', $this->api_user->user_id)->getOne('domains');
```

**After:**
```php
$domain = db()->where('domain_id', $domain_id)->where("(`user_id` = {$this->api_user->user_id} OR `type` = 1)")->where('is_enabled', 1)->getOne('domains');
```

## 🚀 Cara Apply Patch

### Opsi A — Manual Edit (Recommended)

1. Backup original:
   ```bash
   cp /path/to/klikcepat/app/controllers/api/ApiDomains.php \
      /path/to/klikcepat/app/controllers/api/ApiDomains.php.backup
   ```

2. Edit file:
   ```bash
   nano /path/to/klikcepat/app/controllers/api/ApiDomains.php
   ```

3. Apply 3 patches di atas (cari line + replace)

4. (Optional) Clear klikcepat cache:
   ```bash
   rm -rf /path/to/klikcepat/cache/data/*  # adjust path
   ```

### Opsi B — Auto-patch via sed

```bash
cd /path/to/klikcepat/app/controllers/api/

# Backup
cp ApiDomains.php ApiDomains.php.backup

# Patch 1 & 2 (count + select)
sed -i 's|WHERE `user_id` = {$this->api_user->user_id}|WHERE (`user_id` = {$this->api_user->user_id} OR `type` = 1) AND `is_enabled` = 1|g' ApiDomains.php

# Patch 3 (get single)
sed -i 's|->where('"'"'domain_id'"'"', $domain_id)->where('"'"'user_id'"'"', $this->api_user->user_id)|->where('"'"'domain_id'"'"', $domain_id)->where("(`user_id` = {$this->api_user->user_id} OR `type` = 1)")->where('"'"'is_enabled'"'"', 1)|g' ApiDomains.php

# Verify
grep -n "user_id.*OR.*type.*1" ApiDomains.php
```

## 🧪 Verify Patch Works

Test dengan curl (replace TOKEN):

```bash
TOKEN=<sub_user_api_key>
curl -s "https://klikcepat.com/api/domains?results_per_page=100" \
  -H "Authorization: Bearer ${TOKEN}" | python3 -m json.tool
```

**Expected (with patch):** Returns array dengan SEMUA shared domains (klikcepat.vip, klikcepat.cc, klikcepat.lat, klikcepat.com, thymeband.com).

**Without patch:** Returns empty array (cuma domain owned).

## 🤖 BongBot Behavior After Patch

Setelah patch applied, BongBot bisa:
- ✅ Auto-detect domain semua link (no manual config)
- ✅ Accurate per-link domain di List + Edit view
- ✅ Hapus need untuk `KLIKCEPAT_DISPLAY_DOMAIN` setting (bot pake API)

## 🔄 Re-apply After Klikcepat Update

Klikcepat update bakal overwrite vendor code. After update:

```bash
# Re-run sed commands above
# OR restore from backup + apply
cp ApiDomains.php.backup ApiDomains.php
# (then re-apply manually)
```

## ⚠️ Risk Assessment

| Risk | Mitigation |
|---|---|
| Klikcepat update overwrite | Backup + re-apply patch (script-able) |
| Patch breaks if klikcepat changes ApiDomains.php structure | Manual review patch validity |
| Other endpoints rely on user_id filter | `is_enabled = 1` ensures only active shared domains |
| Security: sub-user see other users' domains | NO — only `type = 1` (admin-marked shared) ke-include |

## 🎯 Total Change Impact

- **3 lines** SQL query modification
- **1 file** affected (`ApiDomains.php`)
- **Backward compatible** (existing user-owned domains still returned)
- **Non-breaking** untuk existing klikcepat features

---

Once patched, sub-user API key bisa list semua available domains (owned + shared) — bot auto-detect berfungsi sempurna.
