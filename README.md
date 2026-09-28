# Logwell

Un puits de logs syslog simple : réception UDP/TCP sur le port 514, stockage dans
[VictoriaLogs](https://docs.victoriametrics.com/victorialogs/), et une interface web
claire (thème clair/sombre, recherche dynamique, direct, tags de couleur).

```
équipements ──514 udp/tcp──▶ logwell (Go) ──lots HTTP──▶ VictoriaLogs
                                  ▲    └── SSE (direct) ──▶ navigateur
                                  └──── API / interface ◀──┘
```

## Démarrage

```bash
cp .env.example .env      # optionnel
docker compose up -d --build
./tools/send-test-logs.sh # envoie 100 messages de test
```

Puis ouvrez <http://localhost:8080>.

## Envoyer ses logs

- **rsyslog** (Linux) : ajoutez `*.* @IP_DU_SERVEUR:514` (UDP) ou `*.* @@IP_DU_SERVEUR:514` (TCP)
  dans `/etc/rsyslog.d/90-logwell.conf`, puis exécutez `systemctl restart rsyslog`.
- **Équipements réseau, NAS, pare-feu** : renseignez l'IP du serveur et le port 514 dans la
  configuration « syslog distant ».
- **Test manuel** : `logger -n 127.0.0.1 -P 514 -d "hello error"` (util-linux) ou
  `echo "<14>test ok" | nc -u -w1 127.0.0.1 514`.

Formats acceptés : RFC 3164 (BSD) et RFC 5424. En TCP, les découpages « une ligne par message »
et « octet counting » (RFC 6587) sont tous deux acceptés.

## Recherche

**Mode simple** (par défaut) : chaque mot est recherché comme une sous-chaîne, sans tenir
compte de la casse, dans le message, l'hôte et l'appli. Les mots se combinent avec un ET.

| Saisie | Effet |
|---|---|
| `error disk` | contient « error » **et** « disk » |
| `"disk full"` | contient l'expression exacte |
| `error -timeout` | contient « error » mais pas « timeout » |

**Mode LogsQL** (bouton `Simple` / `LogsQL`) : syntaxe complète de VictoriaLogs, par exemple
`error AND host:web-01`, `app:~"ssh|nginx"`, ou `* | stats by (host) count()`.
Le direct est désactivé dans ce mode.

Raccourcis : `/` place le curseur dans la recherche, `Échap` la vide. Un clic sur une ligne
affiche tous ses champs.

## Tags de couleur

L'icône d'étiquette ouvre l'éditeur. Chaque tag a un mot-clé, une couleur de fond (le texte
passe automatiquement en noir ou en blanc pour rester lisible) et des options : mot entier,
respect de la casse, expression régulière, actif. Les tags sont enregistrés dans
`/data/tags.json` (volume `logwell-data`). Tags par défaut : `warning` (orange),
`error` (rouge), `ok` (vert).

## Configuration

| Variable | Défaut | Rôle |
|---|---|---|
| `SYSLOG_PORT` | `514` | port syslog publié sur l'hôte |
| `HTTP_PORT` | `8080` | port de l'interface web |
| `RETENTION` | `30d` | durée de conservation dans VictoriaLogs |
| `AUTH_USER` / `AUTH_PASS` | vide | authentification HTTP Basic de l'interface |
| `TZ` | `Europe/Paris` | fuseau utilisé pour les horodatages RFC 3164 (sans fuseau) |
| `BATCH_SIZE`, `FLUSH_MS`, `QUEUE_SIZE` | `1000`, `1000`, `100000` | réglages de l'ingestion |

## Débogage

- `docker compose logs -f logwell` : erreurs de réception et d'envoi vers VictoriaLogs.
- La barre du bas affiche les compteurs reçus / stockés / perdus et la dernière erreur de stockage.
- <http://localhost:9428/select/vmui> : interface native de VictoriaLogs pour tester des requêtes LogsQL.
- API :
  ```bash
  curl 'localhost:8080/api/logs?q=error&range=1h&limit=5'   # la réponse inclut la requête LogsQL générée
  curl localhost:8080/api/stats
  curl localhost:8080/api/tags
  ```
- Exécution hors Docker : `VLOGS_URL=http://localhost:9428 DATA_DIR=./data SYSLOG_ADDR=:5514 go run .`

## Structure du code

| Fichier | Contenu |
|---|---|
| `main.go` | configuration, démarrage, authentification |
| `syslog.go` | écoute UDP/TCP et décodage RFC 3164 / 5424 |
| `store.go` | envoi par lots à VictoriaLogs et requêtes LogsQL |
| `query.go` | traduction des filtres de l'interface en LogsQL, filtre du direct |
| `hub.go` | diffusion des nouveaux messages aux navigateurs (SSE) |
| `tags.go` | stockage des tags de couleur |
| `api.go` | routes HTTP `/api/*` |
| `web/` | interface (HTML, CSS, JavaScript sans framework ni build), embarquée dans le binaire |

## Remarque

Sous Docker Desktop (macOS/Windows), l'IP source vue par le conteneur pour les paquets UDP
est celle de la passerelle Docker. Le champ `host` provient toutefois de l'en-tête syslog,
qui contient normalement le vrai nom de l'émetteur.
