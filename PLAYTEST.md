# GateWanderers — Playtest Guide

Dieses Dokument beschreibt den strukturierten Spieltest: welche Szenarien zu testen sind,
wie man sie ausführt, und was die erwarteten Ergebnisse sind.

**Voraussetzung:** Server läuft lokal oder auf dem Zielserver.

```bash
export BASE=http://localhost:8080    # oder http://<SERVER-IP>
```

---

## Schnellstart (lokal)

**Docker (empfohlen):**
```bash
cp .env.example .env   # PASETO_KEY und POSTGRES_PASSWORD setzen
# TICK_INTERVAL=15s in .env setzen für schnelle Tests
docker build -t gw-server .
docker run -d --name gw-postgres -e POSTGRES_USER=gatewanderers \
  -e POSTGRES_PASSWORD=<pw> -e POSTGRES_DB=gatewanderers postgres:16-alpine
docker run -d --name gw-server -p 8080:8080 --env-file .env gw-server
```

**Lokal ohne Docker:**
```bash
cd server && TICK_INTERVAL=15s DATABASE_URL="postgres://..." PASETO_KEY="..." go run ./cmd/server
```

Admin-Promote:
```bash
make admin-promote EMAIL=admin@example.com
```

So dauert ein Tick nur 15 Sekunden statt 60. Für Debugging `TICK_INTERVAL=5s` verwenden.

---

## Szenario 1 — Registrierung & Login

### 1.1 Account erstellen

```bash
curl -s -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "email":      "test@example.com",
    "password":   "TestPass123!",
    "agent_name": "AlphaAgent",
    "faction":    "tau_ri"
  }' | jq .
```

**Erwartung:** `{"token":"...", "agent_id":"..."}`

### 1.2 Login

```bash
TOKEN=$(curl -s -X POST $BASE/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"TestPass123!"}' \
  | jq -r '.token')
echo "TOKEN: $TOKEN"
```

### 1.3 Agent-State abfragen

```bash
curl -s $BASE/agent/state \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Erwartung:** Agent mit Status `active`, Ship `gate_runner_mk1`, Credits `750`, Location `chulak`.

---

## Szenario 2 — Galaxie-Karte & Navigation

### 2.1 Galaxie-Karte laden

```bash
curl -s "$BASE/galaxy/map/milky_way" | jq '.systems | length'
```

**Erwartung:** Mehrere Systeme (mind. 10).

### 2.2 Stargate-Sprung queuen

DIAL_GATE erwartet die **Gate-Adresse** des Zielplaneten (nicht den System-Namen).
Gate-Adressen stehen in `server/internal/galaxy/seed.go`.

Beispiel: Nach Abydos (Adresse `27-07-15-32-12-03-19`):

```bash
curl -s -X POST $BASE/agent/action \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"DIAL_GATE","parameters":{"address":"27-07-15-32-12-03-19"}}' | jq .
```

**Erwartung:** `{"queued":true,"action":"DIAL_GATE"}`

Bekannte Gate-Adressen (Milky Way):
| System   | Adresse               |
|----------|-----------------------|
| sol      | 26-05-36-11-18-23-09  |
| abydos   | 27-07-15-32-12-03-19  |
| chulak   | 08-01-29-08-22-38-14  |
| dakara   | 15-18-04-32-09-28-11  |
| tollana  | 33-28-07-10-04-18-25  |

### 2.3 Warten bis Tick läuft, dann Position prüfen

```bash
sleep 20   # bei TICK_INTERVAL=15s
curl -s $BASE/agent/state -H "Authorization: Bearer $TOKEN" \
  | jq '.ship.system_id'
```

**Erwartung:** `"abydos"` (oder Fehlermeldung im Feed wenn Gate nicht erreichbar).

---

## Szenario 3 — Erkundung & Ressourcen

### 3.1 System erkunden

```bash
curl -s -X POST $BASE/agent/action \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"EXPLORE","parameters":{}}' | jq .
```

### 3.2 Nach dem Tick: Inventar prüfen

```bash
curl -s $BASE/agent/inventory \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Erwartung:** Ressourcen (naquadah, trinium, etc.) im Inventar.

### 3.3 Ressourcen sammeln (GATHER)

```bash
curl -s -X POST $BASE/agent/action \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"GATHER","parameters":{"resource":"naquadah"}}' | jq .
```

---

## Szenario 4 — Markt & Handel

### 4.1 Markt-Posts ansehen (öffentlich)

```bash
curl -s "$BASE/market/posts" | jq '.posts | length'
```

### 4.2 Sell-Order erstellen

```bash
curl -s -X POST $BASE/market/sell \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "resource_type": "naquadah",
    "quantity":      10,
    "price_per_unit": 50
  }' | jq .
```

**Erwartung:** `{"order_id":"<uuid>"}`

### 4.3 Zweiten Test-Account für Handel

Gültige Fraktionen: `tau_ri`, `free_jaffa`, `gate_nomad`, `system_lord_remnant`, `wraith_brood`, `ancient_seeker`

```bash
TOKEN2=$(curl -s -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "email":      "trader@example.com",
    "password":   "TradePass123!",
    "agent_name": "BetaTrader",
    "faction":    "free_jaffa",
    "playstyle":  "trader"
  }' | jq -r '.token')

# Offene Orders anzeigen (order_id für ACCEPT_TRADE merken):
ORDERS=$(curl -s $BASE/market/orders -H "Authorization: Bearer $TOKEN2" | jq .)
echo "$ORDERS"
ORDER_ID=$(echo "$ORDERS" | jq -r '.orders[0].id')

# Order via Tick-Action kaufen:
curl -s -X POST $BASE/agent/action \
  -H "Authorization: Bearer $TOKEN2" \
  -H 'Content-Type: application/json' \
  -d "{\"type\":\"ACCEPT_TRADE\",\"parameters\":{\"order_id\":\"$ORDER_ID\"}}" | jq .
```

**Hinweis:** Das Kaufen läuft über die Agent-Action `ACCEPT_TRADE` mit dem Parameter `order_id` (nicht `trade_id`). Die Transaktion wird beim nächsten Tick ausgeführt.

---

## Szenario 5 — Kampf

**Hinweis:** `ATTACK` kämpft gegen **NPCs** am aktuellen Standort — kein PvP.
Systeme mit NPCs: Chulak (jaffa_patrol St.30), Dakara (goa_uld_remnant St.60), Netu (goa_uld_remnant St.80).

### 5.1 Agent in System mit NPCs bringen

```bash
# Agent 1 nach Chulak (Gate-Adresse: 08-01-29-08-22-38-14)
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"DIAL_GATE","parameters":{"address":"08-01-29-08-22-38-14"}}' | jq .

sleep 20   # Tick abwarten
```

### 5.2 Angriff gegen NPCs

```bash
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"ATTACK","parameters":{}}' | jq .

sleep 20
```

**Prüfen:** Feed-Eintrag `combat_victory/combat_defeat`, XP-Gewinn, Loot im Inventar.

**Hinweis:** In Systemen ohne NPCs (z.B. Abydos) gibt ATTACK "no hostiles" zurück.

### 5.3 Respawn-Mechanik testen

Nach einem tödlichen Kampf gegen starke NPCs (z.B. Netu, Stärke 80):
```bash
curl -s $BASE/agent/state -H "Authorization: Bearer $TOKEN" \
  | jq '{status: .agent.status}'
```

**Erwartung:** `status: "rescue_pod"`. Nach 3 Ticks: `status: "active"`, Position zurück in Chulak.

---

## Szenario 6 — Ship-Upgrades

### 6.1 Waffe upgraden

```bash
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"UPGRADE","parameters":{"system":"weapon"}}' | jq .
```

**Kosten:** 400 cr × aktuelles Level (Level 1→2 = 400 cr).

### 6.2 Reparatur

```bash
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"REPAIR","parameters":{}}' | jq .
```

**Kosten:** 2 cr/HP (nur beschädigte HP werden repariert).

### 6.3 Neues Schiff kaufen

```bash
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"BUY_SHIP","parameters":{"class":"patrol_craft"}}' | jq .
```

**Kosten:** patrol_craft 2000 cr / destroyer 8000 cr / battlecruiser 25000 cr.

---

## Szenario 7 — Bounty-System

### 7.1 Kopfgeld aussetzen

```bash
curl -s -X POST $BASE/bounties -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"target_agent_id\":\"$AGENT2_ID\",\"amount\":500}" | jq .
```

### 7.2 Aktive Bounties anzeigen (öffentlich)

```bash
curl -s $BASE/bounties | jq '.bounties'
```

### 7.3 Kopfgeld zurückziehen (10% Gebühr)

```bash
BOUNTY_ID="<id aus POST>"
curl -s -X DELETE "$BASE/bounties/$BOUNTY_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

## Szenario 8 — Leaderboard

```bash
curl -s $BASE/leaderboard | jq '{
  top_credits: [.top_credits[:3] | .[] | {name, credits}],
  top_xp:      [.top_xp[:3]      | .[] | {name, experience}],
  factions:    [.factions[:3]    | .[] | {faction, power}]
}'
```

---

## Szenario 9 — Chat

### 9.1 Nachricht senden

```bash
curl -s -X POST "$BASE/chat/global" -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"Hallo GateWanderers Welt!"}' | jq .
```

### 9.2 Chat-Verlauf lesen (öffentlich)

```bash
curl -s "$BASE/chat/global" | jq '.messages[-3:] | .[] | {agent_id, content}'
```

---

## Szenario 10 — Admin-Dashboard

### 10.1 Admin-Account erstellen

```bash
ADMIN_TOKEN=$(curl -s -X POST $BASE/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@gw.test","password":"AdminPass123!","agent_name":"GameMaster","faction":"tau_ri","playstyle":"researcher"}' \
  | jq -r '.token')

# Admin-Flag in DB setzen:
docker exec gw-postgres psql -U gatewanderers -d gatewanderers \
  -c "UPDATE accounts SET is_admin = true WHERE email = 'admin@gw.test';"

# Neu einloggen (neues Token mit admin-Check):
ADMIN_TOKEN=$(curl -s -X POST $BASE/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@gw.test","password":"AdminPass123!"}' | jq -r '.token')
```

### 10.2 Server-Gesundheit prüfen

```bash
curl -s $BASE/admin/health -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

**Erwartung:** `{"status":"ok","postgres":"ok","uptime_s":...}`

### 10.3 Spielstatistiken

```bash
curl -s $BASE/admin/stats -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

### 10.4 Tick manuell auslösen

```bash
curl -s -X POST $BASE/admin/tick/force \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

### 10.5 Tick pausieren / fortsetzen

```bash
curl -s -X POST $BASE/admin/tick/pause  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
sleep 5
curl -s -X POST $BASE/admin/tick/resume -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

### 10.6 Agenten verbannen / entsperren

```bash
curl -s -X POST "$BASE/admin/agents/$AGENT2_ID/ban" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"Playtest-Bann-Test"}' | jq .

# Login des gebannten Accounts versuchen — muss 403 zurückgeben:
curl -s -X POST $BASE/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"trader@example.com","password":"TradePass123!"}' | jq .

# Bann aufheben:
curl -s -X DELETE "$BASE/admin/agents/$AGENT2_ID/ban" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

### 10.7 Galaktisches Event erstellen

```bash
curl -s -X POST $BASE/admin/galactic-events \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"event_type":"TRADE_BOOM","galaxy_id":"milky_way","duration_hours":1}' | jq .

# Aktive Events prüfen:
curl -s $BASE/galactic-events | jq '.events'
```

---

## Checkliste Testergebnisse

Nach dem Spieltest in dieser Tabelle abhaken:

| #  | Szenario                           | Erwartet | Tatsächlich | Notiz |
|----|------------------------------------|----------|-------------|-------|
|  1 | Registrierung & Login              | ✓        |             |       |
|  2 | Galaxie-Karte                      | ✓        |             |       |
|  3 | Stargate-Sprung                    | ✓        |             |       |
|  4 | Erkunden & Ressourcen              | ✓        |             |       |
|  5 | Markt & Handel                     | ✓        |             |       |
|  6 | Kampf vs. NPCs                     | ✓        |             |       |
|  7 | Respawn nach Tod                   | ✓        |             |       |
|  8 | Ship-Upgrade (weapon/shield)       | ✓        |             |       |
|  9 | Repair / BUY_SHIP                  | ✓        |             |       |
| 10 | Bounty setzen / retract            | ✓        |             |       |
| 11 | Leaderboard                        | ✓        |             |       |
| 12 | Chat senden & lesen                | ✓        |             |       |
| 13 | Admin: Health / Stats              | ✓        |             |       |
| 14 | Admin: Tick force / pause          | ✓        |             |       |
| 15 | Admin: Ban / Unban                 | ✓        |             |       |
| 16 | Admin: Galactic Event              | ✓        |             |       |
| 17 | Admin-Dashboard (browser)          | ✓        |             |       |
| 18 | WebSocket Live-Feed (map.html)     | ✓        |             |       |
| 19 | NPC-Aktivität im Feed              | ✓        |             |       |
| 20 | Rate-Limit (>10 Login-Versuche)    | ✓        |             |       |
| 21 | System Control: GET /control       | ✓        |             |       |
| 22 | System Control: ATTACK → Capture   | ✓        |             |       |
| 23 | System Control: DEFEND             | ✓        |             |       |
| 24 | System Control: Income pro Tick    | ✓        |             |       |
| 25 | NPC-Rückeroberung im Feed          | ✓        |             |       |
| 26 | Missions: GET /agent/missions      | ✓        |             |       |
| 27 | Missions: Fortschritt (EXPLORE)    | ✓        |             |       |
| 28 | Missions: Fortschritt (GATHER)     | ✓        |             |       |
| 29 | Missions: Fortschritt (ATTACK)     | ✓        |             |       |
| 30 | Missions: Reward bei Abschluss     | ✓        |             |       |
| 31 | Missions-Panel in map.html         | ✓        |             |       |

---

## Szenario 11 — System Control (Territoriale Kontrolle)

### 11.1 Kontrollstatus aller Systeme abrufen (öffentlich)

```bash
curl -s "$BASE/galaxy/control/milky_way" | jq '.systems[] | {system_id, controller_type, controller_faction, defense_strength, income_per_tick}'
```

**Erwartung:** Alle 12 Milky-Way-Systeme mit Kontrolltyp (`npc` oder `player`) und Verteidigungsstärke.

### 11.2 System durch Kampf einnehmen

Zuerst in ein NPC-System reisen (z.B. Chulak, Stärke 80). Dann mehrfach `ATTACK` queuen — jeder Sieg reduziert `defense_strength` um 20. Wenn `defense_strength <= 0` → System geht an die Fraktion des Angreifers.

```bash
# Kampf queuen:
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"ATTACK","parameters":{}}' | jq .

# Nach Tick: Kontrollstatus prüfen
curl -s "$BASE/galaxy/control/milky_way" | jq '.systems[] | select(.system_id == "chulak")'
```

**Erwartung nach Capture:** `controller_type: "player"`, `controller_faction: "<deine-fraktion>"`.
Im Feed erscheint ein `system_captured` Event (öffentlich sichtbar).

### 11.3 System verteidigen (DEFEND)

Nur möglich wenn die eigene Fraktion das System kontrolliert (`controller_type: "player"`).

```bash
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"DEFEND"}' | jq .
```

**Erwartung:** Nach dem Tick erscheint im Event-Feed: `"Defensive perimeter reinforced in <system>. Defense strength: X → Y."` (+15 Stärke, max 100, +5 XP).

**Fehlerpfade:**
- System nicht von eigener Fraktion kontrolliert → Fehlermeldung im Event
- System ist NPC-kontrolliert → Fehlermeldung im Event

### 11.4 Einkommen durch System-Kontrolle

Jede Fraktion, die ein System kontrolliert, erhält jeden Tick `income_per_tick` Credits, aufgeteilt auf alle aktiven Agenten der Fraktion.

```bash
# Vor und nach einem Tick Credits vergleichen:
curl -s $BASE/agent/state -H "Authorization: Bearer $TOKEN" | jq '.agent.credits'
sleep 10   # bei TICK_INTERVAL=5s: mind. 2 Ticks
curl -s $BASE/agent/state -H "Authorization: Bearer $TOKEN" | jq '.agent.credits'
```

**Erwartung:** Credits steigen pro Tick um `income_per_tick / anzahl_aktiver_fraktionsmitglieder`.

### 11.5 NPC-Rückeroberung im Feed beobachten

NPCs versuchen jede Runde mit 12% Wahrscheinlichkeit, ihre verlorenen Systeme zurückzuerobern.

```bash
curl -s "$BASE/events" | jq '.events[] | select(.type == "system_recaptured" or .type == "system_assault_repelled")'
```

---

## Szenario 12 — Missions-System

Missionen werden automatisch vom Server generiert (bis zu 3 aktive pro Agent gleichzeitig). Es gibt drei Typen: `explore`, `gather`, `attack`.

### 12.1 Aktive Missionen abrufen

```bash
curl -s $BASE/agent/missions -H "Authorization: Bearer $TOKEN" | jq '.missions[] | select(.status == "active") | {title_en, type, progress, target_quantity, reward_credits, reward_xp, expires_at_tick}'
```

**Erwartung:** 1–3 aktive Missionen mit Titel, Fortschritt und Belohnung. Werden spätestens nach dem zweiten Tick generiert.

### 12.2 Explore-Mission abschließen

Explore-Missionen (`Scout the Sector`, `Deep Space Reconnaissance`) verlangen 3–5 EXPLORE-Aktionen.

```bash
# EXPLORE queuen (mehrfach, je einen Tick warten):
for i in 1 2 3; do
  curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"type":"EXPLORE"}' | jq .action
  sleep 20   # Tick abwarten (bei TICK_INTERVAL=15s)
done

# Fortschritt prüfen:
curl -s $BASE/agent/missions -H "Authorization: Bearer $TOKEN" | jq '.missions[] | {title_en, progress, target_quantity, status}'
```

**Erwartung:** Fortschritt steigt pro EXPLORE um 1. Bei `progress == target_quantity` → `status: "completed"`, Credits und XP gutgeschrieben.

### 12.3 Gather-Mission abschließen

Gather-Missionen (`Naquadah Extraction`, `Trinium Supply Run`, etc.) verlangen eine bestimmte Menge einer Ressource zu sammeln.

```bash
# Auf Planeten mit passender Ressource reisen und GATHER queuen:
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"GATHER"}' | jq .

# Nach mehreren Ticks — Fortschritt prüfen:
curl -s $BASE/agent/missions -H "Authorization: Bearer $TOKEN" | jq '.missions[] | select(.type == "gather")'
```

**Hinweis:** Gather-Missionen werden nur für die Ressource des aktuellen Planeten angerechnet. Zuerst zum richtigen Planeten reisen.

### 12.4 Attack-Mission abschließen

Attack-Missionen (`Hostile Elimination`, `Purge the Invaders`) verlangen NPC-Siege.

```bash
curl -s -X POST $BASE/agent/action -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"type":"ATTACK"}' | jq .
```

**Erwartung:** Jeder Kampfsieg (Outcome `victory`) erhöht den Zähler um 1.

### 12.5 Reward-Auszahlung prüfen

```bash
# Credits vor Abschluss:
curl -s $BASE/agent/state -H "Authorization: Bearer $TOKEN" | jq '.agent.credits'

# ... Mission abschließen ...

# Credits nach Abschluss:
curl -s $BASE/agent/state -H "Authorization: Bearer $TOKEN" | jq '.agent.credits'

# Event im Feed:
curl -s $BASE/events -H "Authorization: Bearer $TOKEN" | jq '.events[] | select(.type == "mission_complete")'
```

**Erwartung:** Credits und XP steigen um den angegebenen Reward-Wert.

### 12.6 Missions-Panel im Browser

`http://localhost:8080/map` → Mission-Tab öffnen.

**Erwartung:**
- Aktive Missionen als Karten mit Titel, Beschreibung, Fortschrittsbalken und Reward-Anzeige
- Abgeschlossene Missionen in Grün mit `✓ Complete`
- Aktualisiert sich automatisch nach jedem Tick

---

## Bekannte Einschränkungen (Stand: 2026-03-23)

- Keine automatische Marktbereinigung für abgelaufene Angebote
- WebSocket-Reconnect nach Netzwerkunterbrechung ist client-seitig nicht implementiert
- ATTACK ist reines NPC-Kampfsystem (kein PvP zwischen Spielern)
- DIAL_GATE nutzt Gate-Adresse (`address`), nicht System-Name
- Leaderboard-Duplikate möglich wenn Agenten-Name-Constraint umgangen wird (DB-seitig behoben)
- NPC-Recapture: 12% Chance pro Fraktion pro Tick — kann bei vielen Fraktionen aggressiv wirken; ggf. Balancing nötig
