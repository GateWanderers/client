# GateWanderers — Deployment Guide (Debian 13 / Trixie)

## Übersicht

```
Internet → nginx (80/443) → gw-server (8080) → gw-postgres (5432)
```

Alle Dienste laufen in Docker-Containern und werden über `docker compose` verwaltet.

---

## 1. Server vorbereiten (einmalig)

### 1.1 Frischen Debian 13 Server aufsetzen

```bash
# Als root auf dem neuen Server:
sudo bash scripts/init-server.sh
```

Das Skript installiert:
- Docker CE + Docker Compose Plugin
- ufw-Firewall (SSH + 80 + 443 offen)
- fail2ban (Brute-Force-Schutz)
- Swap (wenn RAM < 2 GB)

### 1.2 Repository klonen

```bash
git clone https://github.com/DEIN_USER/gatewanderers.git /opt/gatewanderers
cd /opt/gatewanderers
```

### 1.3 Konfiguration anlegen

```bash
cp .env.example .env
nano .env
```

Pflichtfelder:

| Variable          | Beschreibung                            | Beispiel                        |
|-------------------|-----------------------------------------|---------------------------------|
| `POSTGRES_PASSWORD` | Sicheres DB-Passwort (mindestens 20 Zeichen) | `xK9#mP2&Lq...`         |
| `PASETO_KEY`      | 32-Byte-Zufallsschlüssel (64 Hex-Zeichen) | `openssl rand -hex 32`        |
| `TICK_INTERVAL`   | Game-Engine-Interval (Playtest: `15s`) | `60s`                           |

---

## 2. Deployment

```bash
cd /opt/gatewanderers
bash scripts/deploy.sh
```

Das Skript:
1. Pullt den neuesten Code vom Git-Remote
2. Baut das Docker-Image (multi-stage Go build)
3. Startet Postgres, wartet auf Health-Check
4. Spielt alle SQL-Migrationen ein
5. Startet den Game-Server neu
6. Startet nginx

Nach dem Deployment erreichbar unter:
- **Karte:**  `http://<SERVER-IP>/map`
- **Admin:**  `http://<SERVER-IP>/admin`

---

## 3. Admin-Benutzer erstellen

Zuerst einen normalen Account registrieren (über `/map` oder cURL):

```bash
curl -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"SICHER","agent_name":"AdminAgent","faction":"tau_ri"}'
```

Dann zum Admin befördern:

```bash
make admin-promote EMAIL=admin@example.com
# oder direkt:
docker exec gw-postgres psql -U gatewanderers -d gatewanderers \
  -c "UPDATE accounts SET is_admin = true WHERE email = 'admin@example.com';"
```

---

## 4. TLS / HTTPS mit Let's Encrypt (optional)

### Certbot installieren

```bash
apt-get install -y certbot
certbot certonly --standalone -d deine-domain.de
```

### Zertifikate einbinden

```bash
cp /etc/letsencrypt/live/deine-domain.de/fullchain.pem nginx/certs/
cp /etc/letsencrypt/live/deine-domain.de/privkey.pem  nginx/certs/
```

In `nginx/gatewanderers.conf` die kommentierten TLS-Zeilen aktivieren und HTTP→HTTPS-Redirect einschalten.

```bash
docker compose restart nginx
```

### Auto-Renewal

```bash
echo "0 3 * * * certbot renew --quiet && cp /etc/letsencrypt/live/deine-domain.de/*.pem /opt/gatewanderers/nginx/certs/ && docker compose -f /opt/gatewanderers/docker-compose.yml restart nginx" \
  | crontab -
```

---

## 5. Täglicher Betrieb

### Logs anschauen

```bash
make logs           # nur Server-Logs
make logs-all       # alle Dienste
docker compose logs -f --tail=100 gw-server
```

### Server-Status

```bash
make ps
docker compose ps
```

### Neustart nach Code-Änderung

```bash
git pull
bash scripts/deploy.sh
```

### Nur Migrationen einspielen

```bash
bash scripts/migrate.sh
```

### Datenbank-Shell

```bash
make shell-db
```

### Tick manuell auslösen (Admin-API)

```bash
TOKEN="<admin-token>"
curl -X POST http://localhost:8080/admin/tick/force \
  -H "Authorization: Bearer $TOKEN"
```

---

## 6. Update-Strategie

```bash
cd /opt/gatewanderers
git pull origin main
bash scripts/deploy.sh
```

Der Server wird mit `--force-recreate` neu gestartet (kurze Downtime ~5s).
Für Zero-Downtime wäre ein Load-Balancer mit zwei Server-Instanzen nötig.

---

## 7. Backup

### Datenbank-Dump

```bash
docker exec gw-postgres pg_dump -U gatewanderers gatewanderers \
  | gzip > "backup_$(date +%Y%m%d_%H%M).sql.gz"
```

### Restore

```bash
gunzip -c backup_20260322_1200.sql.gz \
  | docker exec -i gw-postgres psql -U gatewanderers -d gatewanderers
```

### Automatisch täglich

```bash
mkdir -p /opt/backups/gatewanderers
cat > /etc/cron.daily/gw-backup << 'EOF'
#!/bin/bash
docker exec gw-postgres pg_dump -U gatewanderers gatewanderers \
  | gzip > "/opt/backups/gatewanderers/$(date +%Y%m%d).sql.gz"
# Backups älter als 14 Tage löschen
find /opt/backups/gatewanderers -name "*.sql.gz" -mtime +14 -delete
EOF
chmod +x /etc/cron.daily/gw-backup
```

---

## 8. Troubleshooting

### Container startet nicht

```bash
docker compose logs gw-server
# Häufige Ursachen:
# - DATABASE_URL falsch → .env prüfen
# - PASETO_KEY nicht 64 Hex-Zeichen → openssl rand -hex 32
# - Port 8080 bereits belegt → lsof -i :8080
```

### Postgres nicht erreichbar

```bash
docker compose ps gw-postgres
docker inspect gw-postgres | jq '.[0].State.Health'
```

### Migrations-Fehler

```bash
# Einzelne Migration manuell einspielen:
docker cp server/migrations/006_gameplay.sql gw-postgres:/tmp/
docker exec gw-postgres psql -U gatewanderers -d gatewanderers \
  -v ON_ERROR_STOP=1 -f /tmp/006_gameplay.sql
```

### nginx gibt 502 Bad Gateway

Der Server ist noch nicht bereit. Kurz warten, dann:
```bash
make logs
```

---

## 9. Systemanforderungen

| Ressource | Minimum    | Empfohlen     |
|-----------|------------|---------------|
| CPU       | 1 vCore    | 2 vCores      |
| RAM       | 512 MB     | 1–2 GB        |
| Disk      | 5 GB       | 20 GB         |
| OS        | Debian 13  | Debian 13     |
| Docker    | 24+        | 27+           |
