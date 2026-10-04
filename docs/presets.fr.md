[English](presets.md) · **Français**

# Préréglages de tags de couleur

Dans **Paramètres › Motif de recherche**, le menu **+ Préréglage…** ajoute d'un clic un groupe de tags de
couleur tout faits. Les tags ajoutés sont des tags ordinaires : vous pouvez changer leur couleur,
leur motif ou leurs options, ou les supprimer. Un tag dont le motif est déjà dans la liste n'est
pas ajouté en double.

La liste vient d'un fichier texte, [`presets.json`](../presets.json), intégré à LogStream. Vous
pouvez le remplacer par votre propre fichier (voir [Utiliser votre propre fichier](#utiliser-votre-propre-fichier)).

## Préréglages intégrés

### HTTP/HTTPS

Ces préréglages lisent les logs d'accès de nginx et Apache (formats common et combined), Traefik
(CLF et JSON), Caddy (JSON) et HAProxy (`option httplog`).

| Préréglage | Tags | Ce qui est coloré |
| --- | --- | --- |
| Codes HTTP | `HTTP 2xx` vert, `HTTP 3xx` bleu, `HTTP 4xx` orange, `HTTP 5xx` rouge | seulement le code de statut, par exemple `404` dans `"GET /x HTTP/1.1" 404 153`, `"status":404` ou `"DownstreamStatus":404`. Les autres nombres de la ligne (taille, chemin) ne sont pas touchés. |
| Méthodes HTTP | `GET/HEAD/OPTIONS` gris, `POST/PUT/PATCH` violet, `DELETE` rose | seulement la méthode dans `"GET /chemin` ou `"method":"GET"` (en majuscules uniquement) |
| Sondes et attaques | `sondes / attaques` | `wp-login.php`, `xmlrpc.php`, `wp-admin`, `phpmyadmin`, `/.env`, `/.git`, `/.aws`, `/cgi-bin/`, `../`, `%2e%2e`, `/etc/passwd`, `<script`, `union select` |
| Robots et scripts | `robots / scripts` | les mots finissant par `bot` (`Googlebot`, `bingbot`…), `crawler`, `spider`, `curl`, `wget`, `python-requests`, `Go-http-client`, `zgrab`, `masscan`, `nmap`, `sqlmap`, `nikto` |
| Erreurs TLS/HTTPS et proxy | `erreurs TLS`, `erreurs proxy` | échecs de handshake TLS, certificats expirés ou refusés, `x509:` ; `upstream timed out`, `no live upstreams`, `connect() failed`, `connection refused`, `bad gateway`, `gateway timeout`, `service unavailable` |

### Système

| Préréglage | Tags | Ce qui est coloré |
| --- | --- | --- |
| SSH et connexions | `échecs de connexion` rouge, `connexions` vert | sshd/PAM : `Failed password`, `Invalid user`, `authentication failure`, `incorrect password attempts`, `NOT in sudoers`… ; `Accepted publickey`, `session opened for user`, `New session … of user` |
| Commandes sudo | `commandes sudo` | la commande lancée, par exemple `COMMAND=/usr/bin/apt` |
| Noyau : OOM, plantages, disques | `mémoire épuisée`, `erreurs noyau` | `Out of memory`, `oom-killer`, `Killed process 4242` ; `Kernel panic`, `BUG:`, `Oops`, `Call Trace`, `segfault at`, `I/O error`, `EXT4-fs error`, `blocked for more than 120 seconds`, `soft lockup` |
| Services systemd | `services en échec` rouge, `démarrage/arrêt de service` vert | `Failed to start`, `Failed with result`, `Main process exited, code=killed`, `Start request repeated too quickly` ; `Started`, `Stopping`, `Reloaded`, `Reached target` |
| Pare-feu et fail2ban | `pare-feu` | `[UFW BLOCK]`, `[UFW ALLOW]`, `DROP`, `REJECT`, `Ban 203.0.113.9`, `Unban …`, `Found …` |

### Applications

| Préréglage | Tags | Ce qui est coloré |
| --- | --- | --- |
| Docker et conteneurs | `problèmes de conteneur` | `exited with code 137` (codes non nuls seulement), `OOMKilled`, `unhealthy`, `Back-off restarting`, `CrashLoopBackOff`, `container die/kill/oom` |
| Bases de données | `erreurs base de données` | PostgreSQL et MySQL/MariaDB : `deadlock detected`, `duplicate key`, `too many connections`, `lock wait timeout`, `slow query`, `server has gone away`, `Access denied for user`, `password authentication failed`… |

### Général

| Préréglage | Tags | Ce qui est coloré |
| --- | --- | --- |
| Niveaux de log | `fatal / critique` rouge, `info / notice` bleu, `debug / trace` gris, `ok` vert | ces mots en mots entiers, quelle que soit la casse (les tags par défaut `warning` et `error` couvrent le reste) |
| Adresses IPv4 | `adresses IPv4` | `192.168.1.20`, `203.0.113.9`… Les numéros de version à quatre parties comme `1.2.3.4` sont aussi colorés. |

Quand des tags se chevauchent, celui placé le plus haut dans la liste l'emporte : les
préréglages ajoutés après les tags par défaut ne les masquent donc jamais.

## Utiliser votre propre fichier

LogStream lit le fichier indiqué par `PRESETS_FILE`, `/data/presets.json` par défaut (dans le
volume `logstream-data`). S'il n'existe pas, la liste intégrée est utilisée. Le fichier est relu
à chaque ouverture des Paramètres : pas besoin de redémarrer après une modification.

Avec docker-compose, le plus simple est de garder le fichier à côté de `docker-compose.yml` :

1. Copiez [`presets.json`](../presets.json) depuis ce dépôt et modifiez-le.
2. Dans `docker-compose.yml`, décommentez la ligne `- ./presets.json:/config/presets.json:ro`.
3. Dans `.env`, mettez `PRESETS_FILE=/config/presets.json`, puis lancez `docker compose up -d`.

Si le fichier est invalide (erreur JSON, expression régulière ou couleur incorrecte, id en
double), les Paramètres affichent l'erreur et la liste intégrée est utilisée jusqu'à correction.

## Format du fichier

Le fichier est une liste JSON de groupes. Chaque groupe a un nom et une liste de préréglages ;
chaque préréglage a un `id`, un nom et ses tags.

```json
[
  {
    "group": { "en": "My apps", "fr": "Mes applis" },
    "presets": [
      {
        "id": "monappli",
        "name": "Mon appli",
        "tags": [
          { "label": "paiement refusé", "color": "#fca5a5", "pattern": "payment (?:failed|refused)" },
          { "label": "commande", "color": "#86efac", "pattern": "order #\\d+" },
          { "label": "lent", "color": "#fde68a", "pattern": "SLOW", "regex": false, "caseSensitive": true }
        ]
      }
    ]
  }
]
```

| Champ | Obligatoire | Signification |
| --- | --- | --- |
| `group` | oui | nom du groupe dans le menu |
| `id` | oui | identifiant unique du préréglage |
| `name` | oui | nom du préréglage dans le menu |
| `tags[].pattern` | oui | ce qu'il faut colorer : une expression régulière, ou du texte simple avec `"regex": false` |
| `tags[].color` | oui | couleur de fond, `#rrggbb` |
| `tags[].label` | non | nom affiché dans la liste des tags à la place du motif |
| `tags[].regex` | non | `true` par défaut |
| `tags[].wholeWord` | non | mots entiers seulement, `false` par défaut |
| `tags[].caseSensitive` | non | respecter la casse, `false` par défaut |

Les noms et libellés sont soit un seul texte pour toutes les langues (`"Mon appli"`), soit un
texte par langue (`{ "en": "My app", "fr": "Mon appli" }`) ; une langue absente se rabat sur
l'anglais.

Les expressions régulières doivent fonctionner à la fois dans le navigateur (JavaScript) et en
Go, qui les vérifie : évitez les assertions arrière `(?<=…)`, avant `(?=…)` et les références
arrière `\1`. En JSON, chaque barre oblique inverse s'écrit deux fois : `\d` devient `"\\d"`. Un
groupe nommé `hl`, `(?<hl>…)`, ne colore que cette partie de la correspondance, comme le font
les préréglages HTTP avec `(?<hl>5\\d\\d)`.
