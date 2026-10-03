[English](README.md) | Français

# Logstream

Un collecteur syslog simple : il reçoit les logs en UDP/TCP sur le port 514, les stocke dans
[VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) et sert une interface web
soignée (thème clair/sombre, recherche instantanée, direct, tags de couleur, interface en
anglais et en français).

```
devices ──514 udp/tcp──▶ logstream (Go) ──HTTP batches──▶ VictoriaLogs
                              ▲    └── SSE (live) ──▶ browser
                              └──── API / UI ◀─────────┘
```

## Architecture

![Schéma logique de Logstream](docs/architecture.png)

- **Ingestion** : les messages syslog (UDP/TCP) et les logs des conteneurs Docker passent tous
  par `sink()` (résolution DNS inverse des hôtes donnés par leur IP), puis par la file du
  `Store`, qui les envoie par lots à VictoriaLogs.
- **Direct** : `sink()` publie aussi chaque message dans le `Hub`, qui le diffuse aux
  navigateurs en SSE.
- **Recherche** : l'API HTTP traduit les filtres de l'interface en requêtes LogsQL envoyées à
  VictoriaLogs.
- **Frise** : `/api/histogram` (`histogram.go`) compte les logs par intervalle et par sévérité,
  alignés sur l'heure locale ; les bornes du zoom (`from`/`to`) s'appliquent aussi à la liste
  et à l'export.
- **État** : les tags et les réglages des sources sont dans `/data` (volume `logstream-data`) ;
  les logs eux-mêmes dans le volume `vlogs-data`.

La source modifiable du schéma est
[`docs/architecture.excalidraw`](docs/architecture.excalidraw)
(à ouvrir sur [excalidraw.com](https://excalidraw.com)).

## Démarrage

```bash
cp .env.example .env      # optional
docker compose up -d --build
./tools/send-test-logs.sh # sends 100 test messages
```

Ouvrez ensuite <http://localhost:8080>.

## Envoyer des logs

- **rsyslog** (Linux) : ajoutez `*.* @SERVER_IP:514` (UDP) ou `*.* @@SERVER_IP:514` (TCP)
  dans `/etc/rsyslog.d/90-logstream.conf`, puis lancez `systemctl restart rsyslog`.
- **Équipements réseau, NAS, pare-feu** : indiquez l'IP du serveur et le port 514 dans leurs
  réglages « syslog distant » (remote syslog).
- **Test manuel** : `logger -n 127.0.0.1 -P 514 -d "hello error"` (util-linux) ou
  `echo "<14>test ok" | nc -u -w1 127.0.0.1 514`.

**Paramètres > Sources > Syslog** active ou désactive la réception syslog et choisit les
protocoles (UDP, TCP) sans redémarrage ; le choix est enregistré dans `/data/syslog.json`. Cet
onglet affiche aussi l'état d'écoute (et l'erreur si le port est déjà utilisé). Le port
lui-même est publié par docker-compose : modifiez `SYSLOG_PORT` dans `.env`, puis lancez
`docker compose up -d`.

Formats pris en charge : RFC 3164 (BSD) et RFC 5424. En TCP, les deux découpages sont acceptés :
« un message par ligne » et « octet counting » (RFC 6587).

## Recherche

**Mode simple** (par défaut) : chaque mot est cherché comme sous-chaîne, sans tenir compte de
la casse, dans le message, l'hôte et l'application. Les mots sont combinés avec ET.

| Saisie | Signification |
|---|---|
| `error disk` | contient « error » **et** « disk » |
| `"disk full"` | contient la phrase exacte |
| `error -timeout` | contient « error » mais pas « timeout » |

**Mode LogsQL** (bouton `Simple` / `LogsQL`) : le langage de requête complet de VictoriaLogs,
par exemple `error AND host:web-01`, `app:~"ssh|nginx"` ou `* | stats by (host) count()`.
Le direct est désactivé dans ce mode.

Chaque ligne affiche, de gauche à droite : l'**heure de réception** (horloge du serveur),
l'horodatage trouvé dans le message lui-même (`msg_time`), la sévérité, l'hôte, l'application,
les codes des tags trouvés et le message. Un clic sur un hôte ou une application filtre dessus.

Les logs sont indexés, recherchés et triés par **heure de réception** : les équipements dont
l'horloge est fausse (par exemple des points d'accès dont le NTP échoue) apparaissent quand même
dans la bonne plage de temps. Les logs stockés par les versions antérieures à ce changement sont
indexés par l'horodatage de leur message ; purgez-les depuis les Paramètres pour repartir sur
une base propre.

Les badges de sévérité `err`/`crit` et `warning` reprennent les couleurs des tags `error` et
`warning`.

Raccourcis : `/` place le curseur dans la recherche, `Esc` la vide. Un clic sur une ligne
affiche tous ses champs.

Les en-têtes de colonnes restent visibles pendant le défilement. Faites glisser le bord d'un
en-tête (réception, heure message, sévérité, hôte, app) pour redimensionner la colonne, et
double-cliquez dessus pour revenir à la largeur automatique ; le message occupe l'espace
restant. Les largeurs sont mémorisées par le navigateur (**Paramètres > Interface >
Réinitialiser les colonnes** les rétablit toutes). Sur téléphone, la liste garde sa
présentation sur deux lignes, sans colonnes.

## Frise

La frise au-dessus de la liste montre le volume de logs par intervalle, compté selon l'**heure
de réception** sur l'horloge du serveur (elle correspond donc aux heures affichées dans les
lignes).

- Au survol d'un intervalle : ses bornes, son total et le détail par sévérité.
- Un clic sur une barre zoome sur cet intervalle ; un glisser sur plusieurs barres zoome sur la
  sélection. La plage de temps affiche alors la période zoomée (« × Annuler le zoom » ou le
  choix d'une autre plage en sort) ; la liste, les compteurs et l'export CSV suivent le zoom,
  et le direct se met en pause.
- En direct, le dernier intervalle grandit à l'arrivée des messages, et la frise se recharge à
  chaque nouvel intervalle. Rien n'est rafraîchi tant que l'onglet du navigateur est masqué ;
  la frise se met à jour dès qu'il redevient visible.

**Paramètres > Interface > Frise** (mémorisé par navigateur) : échelle (linéaire, √ par défaut,
log), hauteur (S/M/L : 40/80/120 px), couleur (empilement par sévérité, intensité comparée à
la médiane de la fenêtre : calme, rafale au-delà de 3×, anomalie au-delà de 10×, ou aucune),
barres ou aire, division (automatique, environ 100 intervalles, ou fixe : 1 s, 10 s, 1 min,
5 min, 1 h, 1 jour) et rafraîchissement (désactivé, 5 s, 15 s, 30 s, 1 min, ou à chaque nouvel
intervalle). Une division fixe qui dépasserait 300 intervalles sur la plage est élargie
(signalé par « élargi »). Les intervalles sont alignés sur l'heure locale du fuseau horaire
choisi dans les Paramètres (les jours commencent à minuit, heure locale).

API : `curl 'localhost:8080/api/histogram?range=24h&step=auto&tz=Europe/Paris'` renvoie
`step` (ms), `start` (ms), `count`, `now` (horloge du serveur) et les `buckets` non vides
(`i` = numéro d'intervalle, `n` = total, `sev` = nombre par sévérité). Toutes les routes de
logs acceptent aussi `from` / `to` (millisecondes Unix) à la place de `range`.

## Logs des conteneurs Docker

Logstream collecte aussi les logs des conteneurs Docker qui tournent sur la machine où il est
installé (`DOCKER_LOGS=on`, le défaut dans `docker-compose.yml`). Ils sont recherchés, filtrés,
colorés et exportés comme les messages syslog :

- **host** est le nom de l'hôte Docker, **app** le service compose (ou le nom du conteneur), et
  chaque log porte aussi `container`, `container_id`, `image`, `compose_project`,
  `compose_service` et `stream` (stdout/stderr), visibles dans le détail de la ligne.
- Le filtre **Source** n'affiche que les logs syslog ou que les logs Docker ; les lignes Docker
  ont un petit cube devant le nom de l'application, de la couleur de son projet compose.
- La sévérité vient de la ligne elle-même quand l'application l'écrit : JSON
  (`"level":"error"`), logfmt (`level=warn`), `[ERROR]`, ou un niveau en majuscules au début de
  la ligne (`ERROR`, `WARN`…). Sinon, elle vaut `info`. Les codes de couleur du terminal sont
  supprimés.
- **Paramètres > Sources** affiche une étiquette par conteneur (`project/service`) dans un seul
  champ : les conteneurs suivis en couleur, puis les non suivis en gris ; un clic bascule une
  étiquette. La couleur identifie le projet compose, et les noms d'applications Docker de la
  liste utilisent la même couleur. Les conteneurs arrêtés sont masqués par défaut (« Afficher
  les conteneurs arrêtés » les montre, en pointillés, et ils restent modifiables). Une zone de
  filtre et « Tout activer » / « Tout désactiver » (appliqués aux étiquettes affichées) aident
  quand les conteneurs sont nombreux. Les nouveaux conteneurs sont suivis automatiquement, sauf
  si cette option est désactivée. Les choix sont enregistrés par service compose (ou nom de
  conteneur) dans `/data/docker.json` : ils survivent aux recréations.
- Logstream mémorise la position lue dans chaque conteneur (`/data/docker-state.json`) : après
  un redémarrage, il reprend sans perdre ni dupliquer de lignes. Un conteneur vu pour la
  première fois est lu à partir de `DOCKER_BACKFILL` en arrière (1 heure par défaut).
- Logstream lui-même et le proxy ci-dessous ne sont jamais collectés ; ajoutez l'étiquette
  `logstream.exclude=true` à tout autre conteneur pour l'exclure définitivement.

**Sécurité** : l'accès au socket Docker équivaut à un accès root sur la machine. Logstream passe
donc par [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy), qui ne laisse
passer que la liste des conteneurs, la lecture des logs, les événements et les informations du
moteur (`GET` uniquement).

## Logs système de l'hôte

Logstream peut aussi collecter les logs système de la machine qui héberge la stack, sans rien
configurer sur l'hôte. La source est **désactivée par défaut** : activez-la dans
**Paramètres > Sources > Logs système de l'hôte** (enregistré dans `/data/hostlogs.json`).

- `docker-compose.yml` monte `/var/log` et `/run/log/journal` en lecture seule sous `/host`.
- Si l'hôte utilise systemd, Logstream lit directement les fichiers du **journal systemd**
  (pas besoin de `journalctl` dans l'image) : **host** est le nom de la machine, **app** le
  programme (`SYSLOG_IDENTIFIER`), la sévérité et la facility viennent du journal, et l'unité
  systemd est conservée dans `unit`. Sinon, il suit les fichiers texte de `/var/log` (`syslog`,
  `messages`, `*.log`), analysés comme des lignes syslog, avec le nom du fichier dans `log_file`.
- Ces logs ont la source `host` : le filtre **Source** permet de les afficher seuls.
- À l'activation, la dernière heure est lue d'abord (`HOST_LOGS_BACKFILL`) ; la position
  atteinte est enregistrée dans `/data/hostlogs-state.json`, un redémarrage ne perd donc ni ne
  duplique d'entrées.
- **Droits** : le conteneur tourne avec un utilisateur sans privilège et reçoit le groupe `adm`
  (gid 4), qui peut lire le journal et `/var/log` sur Debian et Ubuntu. Sur d'autres systèmes,
  réglez `HOST_LOGS_GID` dans `.env` sur le gid de `systemd-journal`
  (`getent group systemd-journal | cut -d: -f3`). Paramètres > Sources affiche un message clair
  si l'accès est refusé.
- Limites : les champs du journal compressés par journald (messages de plus de 512 octets,
  compressés en zstd/lz4/xz) ne peuvent pas être décodés sans bibliothèque supplémentaire ; ils
  sont comptés dans les Paramètres et affichés comme « (compressed journal entry) ». Les fichiers
  texte tournés ou compressés (`*.1`, `*.gz`) ne sont pas lus.

## Export CSV

Le bouton **Exporter** (à côté du nombre de logs) télécharge tous les logs stockés qui
correspondent aux filtres en cours (recherche, plage de temps, sévérité, hôte, application),
du plus récent au plus ancien, jusqu'à `EXPORT_MAX` lignes (100 000 par défaut), et pas
seulement les lignes à l'écran. Deux variantes :

- **CSV** : séparateur virgule, UTF-8.
- **CSV pour Excel** : séparateur point-virgule avec un BOM UTF-8, pour qu'Excel en français
  l'ouvre directement avec les accents. Les cellules qui commencent par `=`, `+`, `-` ou `@`
  sont préfixées par `'` pour qu'un message de log forgé ne puisse pas s'exécuter comme une
  formule.

Colonnes : `received`, `message_time` (toutes deux au format `YYYY-MM-DD HH:MM:SS.mmm` dans le
fuseau horaire choisi dans les Paramètres), `severity`, `facility`, `host`, `host_ip`, `app`,
`pid`, `source_ip`, `proto`, `message`. En mode LogsQL, seule la partie filtre est prise en
charge (pas de `| pipes`).

En ligne de commande : `curl -o logs.csv 'localhost:8080/api/export.csv?q=error&range=24h&tz=Europe/Paris'`.

## Paramètres

L'icône en forme d'engrenage ouvre les paramètres, organisés en onglets. Tout, sauf les tags de
couleur, est mémorisé par navigateur.

- **Localisation**
  - *Langue* : anglais ou français.
  - *Date et heure* : fuseau horaire (celui du navigateur, UTC ou environ 80 fuseaux courants)
    et format d'affichage de l'heure de réception : `DD/MM/YYYY HH:MM:SS` (par défaut,
    l'affichage français habituel), avec millisecondes, `YYYY-MM-DD`, horloge sur 12 heures,
    ISO 8601 ou epoch Unix. Le fuseau horaire s'applique à toutes les dates affichées.
- **Filtres** : tags de couleur. Chaque tag a un mot-clé, une couleur de fond (le texte passe
  automatiquement en noir ou en blanc pour rester lisible) et des options : mot entier, respect
  de la casse, expression régulière, actif. Les tags sont stockés sur le serveur dans
  `/data/tags.json` (volume `logstream-data`) : ils sont donc partagés par tous les navigateurs.
  Tags par défaut (pastel) : `warning` (orange), `error` (rouge), `ok` (vert). Les tags par
  défaut qui utilisent encore les couleurs des versions précédentes passent automatiquement aux
  couleurs pastel.
  Le menu *+ Préréglage…* ajoute des tags tout faits : logs d'accès HTTP/HTTPS (codes de statut,
  méthodes, sondes, robots, erreurs TLS et proxy), logs système (SSH, sudo, noyau, systemd,
  pare-feu), applications (Docker, bases de données) et motifs généraux (niveaux de log,
  adresses IPv4). Les tags déjà présents ne sont pas ajoutés en double, et les tags ajoutés se
  modifient comme les autres. La liste vient d'un fichier texte modifiable (`PRESETS_FILE`) :
  voir [docs/presets.fr.md](docs/presets.fr.md) pour le détail de chaque préréglage et le
  format du fichier. Dans une expression
  régulière, un groupe nommé `hl` (`(?<hl>…)`) ne colore que cette partie de la correspondance :
  les préréglages s'en servent pour colorer le code de statut ou la méthode, pas le texte autour.
  Chaque tag reçoit un code à deux chiffres (`01`, `02`…) attribué par le serveur : il reste
  attaché au tag jusqu'à sa suppression (les tags créés par les versions précédentes en reçoivent
  un aussi). La colonne *Filtres* de la liste affiche, en badges gris, les codes des tags actifs
  trouvés dans chaque message ; elle a la place pour 3, au-delà elle en affiche 2 et `+N`, et
  l'infobulle les liste tous.
- **Interface**
  - *Thème* : Système (suit la préférence de l'ordinateur ou du téléphone), Clair ou Sombre. Le
    bouton soleil/lune de l'en-tête bascule entre clair et sombre.
  - *Affichage des logs* : taille du texte (très petite, petite, moyenne, grande), densité
    (normale, ou compacte pour afficher environ 50 % de lignes en plus à l'écran) et police :
    la police monospace du système, l'une des 3 polices étroites intégrées, servies par
    LogStream lui-même et utilisables hors ligne (Inconsolata Condensed, la plus étroite,
    Iosevka et Ubuntu Mono), ou l'une des 11 polices libres conçues pour le texte dense
    (JetBrains Mono, Fira Code, Source Code Pro, IBM Plex Mono, Cascadia Code, Roboto Mono,
    Inconsolata, Red Hat Mono, Noto Sans Mono, Victor Mono, DM Mono). Ces 11 polices sont
    chargées par le navigateur depuis [Bunny Fonts](https://fonts.bunny.net), un service
    européen de polices respectueux de la vie privée ; sans accès à internet, la police du
    système est utilisée. Les ligatures sont désactivées pour que `->` ou `!=` s'affichent tels
    quels. Le réglage le plus dense est Très petite + Compacte + Inconsolata Condensed.
- **Données** : « Supprimer tous les logs » efface définitivement tous les logs stockés (il faut
  taper `PURGE` pour confirmer). Les tags et les paramètres sont conservés. VictoriaLogs doit
  être lancé avec `-delete.enable` (déjà présent dans `docker-compose.yml`) ; mettez
  `ALLOW_PURGE=false` pour désactiver la fonction. Toute personne qui peut ouvrir l'interface
  peut purger : activez l'[authentification](#authentification) si l'interface est accessible
  à d'autres.

## Authentification

`AUTH_MODE` choisit comment l'interface et l'API sont protégées (`/healthz` reste toujours ouvert) :

- **`local`** (par défaut) : une page de connexion avec le compte `AUTH_USER` / `AUTH_PASS` ; laissez-les
  vides pour n'avoir aucune authentification (par exemple derrière un reverse proxy qui contrôle déjà).
- **`oidc`** : connexion par un fournisseur OpenID Connect (Keycloak, Authentik, Authelia, Zitadel…),
  flux « authorization code » avec PKCE.

En mode `local`, la page de connexion suit le thème et la langue de l'interface. La session dure
`SESSION_TTL` (12 h par défaut), survit aux redémarrages (sa clé de signature est dans
`/data/session.key`) et se termine quand `AUTH_USER` ou `AUTH_PASS` change ; le bouton de
déconnexion (en haut à droite) y met fin. Les échecs de connexion sont écrits dans les logs avec
l'adresse du client (`auth: failed login for "bob" from 192.0.2.7`). Les scripts peuvent toujours
appeler l'API avec des identifiants HTTP Basic (`curl -u utilisateur:motdepasse`).

Pour afficher votre logo sur la page de connexion, montez un PNG dans le conteneur et indiquez
son chemin dans `LOGIN_LOGO` :

```yaml
# docker-compose.yml, service logstream
    volumes:
      - ./logo.png:/config/logo.png:ro
```
```bash
# .env
LOGIN_LOGO=/config/logo.png
```

Pour utiliser OIDC :

1. Dans le fournisseur, créez un client **confidentiel** (avec secret) pour logstream et déclarez
   l'URL de retour `https://logs.example.org/auth/callback` (votre adresse).
2. Dans `.env` :
   ```bash
   AUTH_MODE=oidc
   OIDC_ISSUER=https://sso.example.org/realms/maison    # exactement l'« issuer » du fournisseur
   OIDC_CLIENT_ID=logstream
   OIDC_CLIENT_SECRET=...
   OIDC_REDIRECT_URL=https://logs.example.org/auth/callback
   ```
3. `docker compose up -d`. Les logs affichent `oidc authentication enabled`, ou la raison pour
   laquelle le fournisseur n'a pas pu être lu (issuer incorrect, injoignable…).

Ouvrir l'interface renvoie vers la page de connexion du fournisseur, puis revient sur logstream.
La session dure `SESSION_TTL` (12 h par défaut) et survit aux redémarrages (sa clé de
signature est dans `/data/session.key`) ; à son expiration, la page repasse par la connexion. Le
bouton de déconnexion (en haut à droite) termine la session logstream, puis ouvre la page de
déconnexion du fournisseur s'il en a une.

Tout utilisateur accepté par le fournisseur pour ce client peut se connecter : restreignez l'accès
dans le fournisseur (Keycloak : rôles du client ou realm dédié ; Authentik : liaisons de
l'application). Les connexions sont écrites dans les logs de logstream (`oidc: alice logged in`).
Avec une URL de retour en `https`, les cookies ne sont envoyés qu'en HTTPS : logstream doit être
joint à travers un reverse proxy TLS.

## Noms d'hôtes (DNS inverse)

Quand un équipement envoie son adresse IP comme nom d'hôte (ou pas de nom d'hôte du tout),
Logstream cherche son nom DNS (enregistrement PTR) et stocke le nom dans `host` et l'IP dans
`host_ip`. Les résultats sont mis en cache (1 heure, 10 minutes quand il n'y a pas de nom). Les
logs stockés auparavant avec une IP sont résolus à l'affichage, et le filtre d'hôte affiche
`name (IP)`.

Le conteneur utilise le DNS de Docker, qui relaie vers les résolveurs de l'hôte. Si vos noms
locaux ne sont connus que de votre routeur ou d'un DNS local (Pi-hole, AdGuard, Unbound…),
définissez `DNS_SERVER=192.168.1.1` (son adresse). Mettez `RDNS=off` pour désactiver les
résolutions.

## Configuration

| Variable | Défaut | Rôle |
|---|---|---|
| `SYSLOG_PORT` | `514` | port syslog publié sur l'hôte |
| `HTTP_PORT` | `8080` | port de l'interface web |
| `RETENTION` | `30d` | durée de conservation des logs dans VictoriaLogs |
| `AUTH_MODE` | `local` | `local` (page de connexion) ou `oidc`, voir [Authentification](#authentification) |
| `AUTH_USER` / `AUTH_PASS` | vide | compte de la page de connexion (mode `local`) ; vide = pas d'authentification |
| `LOGIN_LOGO` | vide | PNG affiché sur la page de connexion, chemin dans le conteneur (mode `local`) |
| `SESSION_TTL` | `12h` | durée de la session (les deux modes ; `OIDC_SESSION_TTL` fonctionne toujours) |
| `OIDC_ISSUER` | vide | URL de l'issuer du fournisseur OpenID Connect (mode `oidc`) |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | vide | client déclaré dans le fournisseur |
| `OIDC_REDIRECT_URL` | vide | URL de retour de logstream, ex. `https://logs.example.org/auth/callback` |
| `OIDC_SCOPES` | `openid profile email` | scopes demandés |
| `RDNS` | `on` | résoudre les hôtes donnés par leur IP en noms DNS |
| `DNS_SERVER` | vide | serveur DNS pour les résolutions inverses (`ip` ou `ip:port`) |
| `ALLOW_PURGE` | `true` | autoriser « Supprimer tous les logs » dans les Paramètres |
| `EXPORT_MAX` | `100000` | nombre maximal de lignes dans un export CSV |
| `PRESETS_FILE` | `/data/presets.json` | fichier des préréglages de tags ; liste intégrée s'il est absent (voir [docs/presets.fr.md](docs/presets.fr.md)) |
| `DOCKER_LOGS` | `on` dans compose | collecter les logs des conteneurs Docker locaux |
| `DOCKER_HOST` | `tcp://docker-proxy:2375` dans compose | adresse de l'API Docker (`unix:///var/run/docker.sock` hors compose) |
| `DOCKER_BACKFILL` | `1h` | historique lu pour un conteneur vu pour la première fois |
| `HOST_LOGS_GID` | `4` (adm) dans compose | groupe donné au conteneur pour lire les logs de l'hôte |
| `HOST_LOGS_BACKFILL` | `1h` | historique lu à l'activation de la source « logs système de l'hôte » |
| `HOST_LOGS_ROOT` | `/host` | emplacement de montage des répertoires de l'hôte |
| `TZ` | `Europe/Paris` | fuseau horaire des horodatages RFC 3164 (qui n'en portent pas) |
| `BATCH_SIZE`, `FLUSH_MS`, `QUEUE_SIZE` | `1000`, `1000`, `100000` | réglage de l'ingestion |

## Débogage

- `docker compose logs -f logstream` : erreurs de réception et erreurs d'envoi vers
  VictoriaLogs.
- La barre du bas affiche les compteurs reçus / stockés / perdus et la dernière erreur de
  stockage.
- <http://localhost:9428/select/vmui> : l'interface de VictoriaLogs, pour essayer des requêtes
  LogsQL.
- API :
  ```bash
  curl 'localhost:8080/api/logs?q=error&range=1h&limit=5'   # the response includes the generated LogsQL query
  curl localhost:8080/api/stats
  curl localhost:8080/api/tags
  ```
- Lancement hors Docker (Go 1.22+) : `VLOGS_URL=http://localhost:9428 DATA_DIR=./data SYSLOG_ADDR=:5514 go run .`

## Mise à jour

Toutes les versions d'images sont épinglées : un `docker compose pull` ou une reconstruction ne
change jamais un composant à votre insu.

| Emplacement | Image | Version |
|---|---|---|
| `docker-compose.yml` | `victoriametrics/victoria-logs` | `v1.52.0` |
| `docker-compose.yml` | `tecnativa/docker-socket-proxy` | `v0.5.0` |
| `Dockerfile` (build) | `golang` | `1.27.1-alpine3.24` |
| `Dockerfile` (exécution) | `alpine` | `3.24.2` |

Pour mettre à jour l'une d'elles :

1. Lisez les notes de version : [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/changelog/),
   [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy/releases),
   [Go](https://go.dev/doc/devel/release), [Alpine](https://alpinelinux.org/releases/).
   VictoriaLogs conserve son format de stockage entre versions mineures ; lisez le changelog
   avant un changement de version majeure.
2. Modifiez la version dans le fichier indiqué ci-dessus, puis lancez `docker compose up -d --build`.
3. Vérifiez la barre du bas (reçus / stockés / perdus) et **Paramètres > Sources**. Pour revenir
   en arrière, remettez la version précédente et lancez la même commande.

## Organisation du code

| Fichier | Contenu |
|---|---|
| `main.go` | configuration, démarrage |
| `auth.go` | authentification : OpenID Connect (découverte, PKCE, contrôle de l'ID token) et cookie de session signé |
| `auth_local.go` | mode `local` : page de connexion (`web/login.html`), cookie de session, `LOGIN_LOGO` |
| `syslog.go` | écoute UDP/TCP et analyse RFC 3164 / 5424 |
| `store.go` | insertions par lots dans VictoriaLogs et requêtes LogsQL |
| `query.go` | traduit les filtres de l'interface en LogsQL ; filtre du direct |
| `histogram.go` | frise : division, alignement des intervalles, `/api/histogram` |
| `hub.go` | envoie les nouveaux messages aux navigateurs (SSE) |
| `rdns.go` | résolutions DNS inverses avec cache |
| `export.go` | export CSV en flux |
| `docker.go` | logs des conteneurs Docker (API, lecteurs, positions, détection du niveau) |
| `syslogserver.go` | écoutes syslog ouvertes et fermées depuis Paramètres > Sources |
| `hostlogs.go`, `journal.go` | logs système de l'hôte : lecteur du journal systemd (sans `journalctl`) et suivi de `/var/log` |
| `tags.go` | stockage des tags de couleur |
| `presets.go`, `presets.json` | préréglages de tags (`/api/presets`), liste intégrée |
| `api.go` | routes HTTP `/api/*` |
| `web/` | interface (HTML, CSS, JavaScript simple, sans étape de build), embarquée dans le binaire ; les traductions sont dans `web/app.js` (`I18N`), et dans `web/login.html` pour la page de connexion |

## Remarque

Avec Docker Desktop (macOS/Windows), l'IP source vue par le conteneur pour les paquets UDP est
la passerelle Docker. Le champ `host` vient toujours de l'en-tête syslog, qui porte normalement
le vrai nom de l'émetteur.
