'use strict';

/* ================= Translations ================= */

const I18N = {
  en: {
    locale: 'en-US',
    searchAria: 'Search logs',
    searchSimple: 'Search…  error -timeout "disk full"   (press /)',
    searchLogsql: 'LogsQL: error AND host:web-01 | sort by (_time)',
    modeTitle: 'Switch between simple search and LogsQL',
    live: 'Live',
    liveTitle: 'Show new messages live',
    liveUnavailable: 'Live view is not available in LogsQL mode',
    settings: 'Settings',
    theme: 'Light / dark theme',
    logout: 'Log out',
    close: 'Close',
    rangeAria: 'Time range', severityAria: 'Severity', hostAria: 'Host', appAria: 'Application',
    histoAria: 'Log volume over time',
    r5m: '5 min', r15m: '15 min', r1h: '1 hour', r6h: '6 hours', r24h: '24 hours', r7d: '7 days', r30d: '30 days', rAll: 'All',
    sevAll: 'All severities', sev2: 'Critical and above', sev3: 'Errors and above',
    sev4: 'Warnings and above', sev5: 'Notice and above', sev6: 'Info and above',
    allHosts: 'All hosts', allApps: 'All apps',
    emptyTitle: 'No matching logs.',
    emptyHint: 'Widen the time range or send syslog messages to port 514 (UDP or TCP).',
    count: (n) => `${n} ${n === 1 ? 'log' : 'logs'} shown`,
    histoMax: (n) => `max ${n} / interval`,
    searchError: 'Search error: ',
    connLive: 'live', connConnecting: 'connecting…', connRetry: 'reconnecting…',
    connPaused: 'live paused', connLogsql: 'live unavailable (LogsQL)',
    newLogs: (n) => `↑ ${n} new ${n === 1 ? 'log' : 'logs'}`,
    skipped: (n) => `${n} messages not shown (rate too high)`,
    stats: (s) => `received ${s.received} · stored ${s.ingested} · dropped ${s.dropped} · queue ${s.queue}`,
    storageErr: 'storage: ',
    spooled: (n) => `${n} waiting on disk`,
    unreachable: 'server unreachable',
    fTime: 'timestamp', fMsg: 'message', fPid: 'pid', fSd: 'structured data',
    filterHost: 'Filter on this host', filterApp: 'Filter on this app',
    copyMsg: 'Copy message', copyJson: 'Copy as JSON', copied: 'Copied',
    language: 'Language',
    tagsTitle: 'Color tags',
    tagsHelp: 'Keywords are highlighted in messages. Changes are saved automatically. When tags overlap, the one highest in the list wins.',
    addTag: '+ Add tag', resetTags: 'Restore default tags',
    optWhole: 'whole word', optWholeTitle: 'Only color whole words (avoids "ok" in "token")',
    optCaseTitle: 'Match case', optRegexTitle: 'Regular expression',
    optActive: 'active', optActiveTitle: 'Enable this tag',
    colorAria: 'Color', keyword: 'keyword', preview: 'preview', del: 'Delete', noTags: 'No tags.',
    newTag: 'new',
    confirmDelete: (p) => `Delete tag "${p}"?`,
    confirmReset: 'Replace all tags with the defaults (warning, error)?',
    presetAria: 'Add a preset', presetPick: '+ Preset…',
    tagFilter: 'Filter tags (keyword, label or code)',
    tagCount: (n) => (n.shown === n.total ? `${n.total} tag${n.total === 1 ? '' : 's'}` : `${n.shown} / ${n.total} tags`),
    noTagMatch: 'No tag matches the filter.',
    presetAdded: (n) => (n ? `${n} tag(s) added` : 'These tags are already in the list'),
    presetsFileErr: (f) => `Invalid presets file ${f}, built-in presets used: `,
    tagsLoadErr: 'Tags: ',
    err_pattern_required: 'The keyword is required',
    err_invalid_color: 'Invalid color (expected #rrggbb)',
    err_invalid_regex: 'Invalid regular expression',
    err_tag_not_found: 'Tag not found',
    err_too_many_tags: 'Too many tags (99 at most)',
    err_live_logsql: 'Live view is not available in LogsQL mode',
    dateTime: 'Date & time',
    tzLabel: 'Time zone',
    fmtLabel: 'Reception time format',
    tzBrowser: (z) => `Browser time zone (${z})`,
    tzGroups: { General: 'General', Europe: 'Europe', Africa: 'Africa', America: 'Americas', Asia: 'Asia', Oceania: 'Oceania & Pacific', Other: 'Atlantic & Indian Ocean' },
    fmtEpoch: 'Unix epoch (milliseconds)',
    timePreview: 'Preview: ',
    timeHelp: 'The reception time comes from the server clock. The time zone applies to every date shown.',
    fReceived: 'received',
    rcvTitle: 'Received by the server',
    msgTimeTitle: 'Timestamp from the message',
    fHostName: 'host name (DNS)', fHostIp: 'host IP',
    clickHost: 'Click to filter on this host', clickApp: 'Click to filter on this app',
    sourceAria: 'Source', srcAll: 'All sources', tabSources: 'Log sources',
    srcHost: 'Host system',
    srcKernel: 'Kernel (dmesg)',
    auditTitle: 'Linux audit (auditd)',
    auditEnabled: 'Collect the audit events of this machine',
    auditPath: 'File',
    auditOff: 'Off: the audit log of the machine hosting LogStream is not read.',
    auditOk: ({ p, r }) => `Following ${p}: ${r} events since startup.`,
    audit_not_found: 'Audit log not found: mount /var/log/audit in the container (see docker-compose.yml) and check the path. ',
    audit_permission: 'Permission denied on the audit log: set log_group in /etc/audit/auditd.conf, or add an ACL for UID 10001 (see the README). ',
    audit_read: 'Cannot read the audit log: ',
    auditHelp: 'Reads the log of the Linux audit daemon (auditd): logins, sudo, watched files, syscalls matched by your rules… The records of one event are grouped in one line, with a summary (who, what, result) and the raw record below it; failures are warnings. When turned on, the last hour is read first (AUDIT_BACKFILL); rotations are followed and reading resumes where it stopped.',
    selfTitle: 'LogStream internal logs',
    selfEnabled: 'Record the events of LogStream itself',
    selfLevel: 'Minimum level',
    selfOff: 'Off: the events of LogStream only go to its container output.',
    selfOk: ({ n }) => `${n} events recorded since startup (host logstream).`,
    selfDropped: (n) => ` ${n} left out (repeated or too many at once).`,
    selfHelp: 'Turns the events of LogStream into logs (host logstream, app = component): VictoriaLogs unreachable and back, lost batches, syslog listener errors, Docker streams lost, failing sources, start and stop. The same event is recorded at most once every 30 s, with the number of repeats.',
    hostTitle: 'Host system logs',
    hostEnabled: 'Collect the system logs of this machine',
    hostOff: 'Off: the system logs of the machine hosting LogStream are not collected.',
    hostWaiting: 'Starting…',
    hostJournal: ({ n, r }) => `Reading the systemd journal (${n} files): ${r} entries since startup.`,
    hostFiles: ({ n, r }) => `Following ${n} files of /var/log: ${r} lines since startup.`,
    hostCompressed: (n) => ` ${n} compressed fields skipped (long messages, readable with journalctl).`,
    host_not_mounted: 'No host log directory found: mount /var/log and /run/log/journal under /host (see docker-compose.yml).',
    host_permission: 'Permission denied: give the container a group allowed to read the logs (HOST_LOGS_GID in .env, see the README). ',
    host_empty: 'No systemd journal and no log file found in /var/log.',
    host_read: 'Cannot read the host logs: ',
    hostHelp: 'Reads the systemd journal of the host (/var/log/journal, /run/log/journal), or the text files of /var/log (syslog, messages, *.log) when there is no journal. Directories are mounted read-only. When turned on, the last hour is read first (HOST_LOGS_BACKFILL).',
    dmesgTitle: 'Kernel messages (dmesg)',
    dmesgEnabled: 'Collect the kernel messages of this machine',
    dmesgOff: 'Off: the kernel messages of the machine hosting LogStream are not collected.',
    dmesgOk: ({ d, r }) => `Reading ${d}: ${r} messages since startup.`,
    dmesgLost: (n) => ` ${n} messages were overwritten in the kernel buffer before being read.`,
    dmesg_not_found: 'Device /dev/kmsg not found in the container: uncomment its devices section in docker-compose.yml (see the README).',
    dmesg_permission: 'Permission denied on /dev/kmsg: set kernel.dmesg_restrict=0 on the host, or run the container as root with the SYSLOG capability (see the README). ',
    dmesg_read: 'Cannot read the kernel messages: ',
    dmesgHelp: 'Reads the kernel ring buffer of the host through /dev/kmsg, like dmesg: hardware, drivers, OOM killer, firewall… When turned on, the last hour is read first (DMESG_BACKFILL); after a restart, reading resumes where it stopped.',
    syslogEnabled: 'Receive syslog messages', syslogProtocols: 'Protocols', syslogPort: 'Port',
    syslogListening: ({ port, protos }) => `Listening on port ${port} (${protos}).`,
    syslogOff: 'Syslog reception is off: syslog messages are ignored (Docker logs are still collected).',
    syslogNone: 'No protocol selected: syslog messages are ignored.',
    syslogErr: (p) => `Cannot listen over ${p}: `,
    syslogPortHelp: (i) => `Published by docker-compose (container port ${i}). To change it, set SYSLOG_PORT in .env, then run docker compose up -d.`,
    dockerTitle: 'Docker containers',
    dockerDefault: 'Follow new containers automatically',
    dockerAll: 'Enable all', dockerNone: 'Disable all',
    dockerOff: 'The Docker source is disabled on this server. Set DOCKER_LOGS=on (see docker-compose.yml).',
    dockerErr: 'Cannot reach Docker: ',
    dockerOk: ({ h, n, f }) => `Connected to Docker on ${h}: ${n} containers, ${f} followed.`,
    dockerStopped: (n) => `Show stopped containers (${n})`,
    dockerFilter: 'Filter containers',
    dockerLegend: 'Colored: followed · grey: not followed · dashed: stopped · the color identifies the compose project. Click a label to switch it.',
    dockerNoMatch: 'No container',
    stFollowing: 'followed', stSelected: 'followed when running', stIgnored: 'not followed', stStopped: 'stopped',
    stLocked: 'excluded', stLockedTitle: 'LogStream itself, or label logstream.exclude=true',
    dockerHelp: 'Logs are read through docker-socket-proxy, a read-only gateway: LogStream can list containers and read their logs, nothing else. Choices apply per compose service (or container name), so they survive container re-creations.',
    fContainer: 'container', fContainerId: 'container ID', fImage: 'image', fProject: 'compose project',
    fService: 'compose service', fStream: 'stream', fSourceType: 'source',
    fUnit: 'systemd unit', fLogFile: 'log file',
    export: 'Export', exportCsvHint: 'Comma separated, UTF-8',
    exportExcel: 'CSV for Excel', exportExcelHint: 'Semicolon separated, for Excel in French',
    exportNote: (n) => `Up to ${n} logs matching the current filters, newest first. Dates use the time zone chosen in Settings.`,
    exporting: 'Exporting…', exportDone: (f) => `Downloaded: ${f}`, exportError: 'Export failed: ',
    err_export_pipes: 'Export does not support LogsQL pipes (| …): keep only the filter part',
    tabLocale: 'Localization', tabFilters: 'Patterns', tabInterface: 'Appearance', tabData: 'Database',
    themeTitle: 'Theme', themeSystem: 'System', themeLight: 'Light', themeDark: 'Dark',
    themeHelp: 'System follows the light/dark preference of your computer or phone.',
    logDisplay: 'Log display', fontSize: 'Font size',
    sizeTiny: 'Tiny', sizeSmall: 'Small', sizeMedium: 'Medium', sizeLarge: 'Large',
    density: 'Density', densityNormal: 'Normal', densityCompact: 'Compact',
    fontLabel: 'Font', fontSystem: 'System monospace (no download)', fontBuiltin: 'built in, narrow',
    fontHelp: 'Free fonts. The built-in ones (Inconsolata Condensed, the narrowest, Iosevka and Ubuntu Mono) are served by LogStream itself and work offline; they are narrow, so more text fits on each line. The others are loaded by your browser from Bunny Fonts, a privacy-friendly European font service; without internet access, the system font is used.',
    previewMsgs: ['error: link down on eth1, carrier lost', 'warning: disk /dev/sda temperature 58°C', 'backup finished: ok (12.4 GB in 00:14:32)'],
    dangerZone: 'Danger zone',
    dbTitle: 'Database', dbRefresh: 'Refresh', dbLoading: 'Loading…',
    dbUnits: ['B', 'KB', 'MB', 'GB', 'TB'],
    dbRows: 'Stored lines', dbDisk: 'Size on disk', dbIndex: (x) => `including ${x} of index`,
    dbRaw: 'Raw size', dbRatio: (x) => `compressed ×${x}`,
    dbPeriod: 'Period', dbFromTo: (p) => `${p.from} → ${p.to}`, dbDays: (n) => `${n} day${n > 1 ? 's' : ''}`,
    dbRetention: (r) => `retention ${r}`,
    dbRecent: 'Last 24 h', dbLastHour: (n) => `${n} in the last hour`,
    dbSources: 'Sources', dbHostsApps: (p) => `${p.h} host${p.hn > 1 ? 's' : ''} · ${p.a} app${p.an > 1 ? 's' : ''}`,
    dbFree: 'Free disk space', dbEmpty: 'The database is empty.',
    dbQueryErr: 'Some figures could not be computed: ',
    purgeHelp: 'Permanently delete every stored log to start from scratch. Color tags and settings are kept.',
    purgeBtn: 'Delete all logs…',
    purgePrompt: 'This permanently deletes ALL stored logs.\n\nType PURGE to confirm:',
    purgeStarted: 'Deletion started',
    purgeRunning: 'Deletion in progress… logs disappear gradually.',
    purgeDone: 'All logs have been deleted.',
    purgeForbidden: 'Purging is disabled on this server (ALLOW_PURGE=false).',
    err_purge_unavailable: 'VictoriaLogs refuses deletions: start it with -delete.enable (see docker-compose.yml)',
    err_purge_forbidden: 'Purging is disabled on this server (ALLOW_PURGE=false)',
    err_read_only: 'Read-only account: changes are reserved to administrators',
    err_cross_site: 'Request refused: it comes from another site',
    authOff: 'No authentication: anyone who can reach this page can read the logs and change the settings. Set AUTH_USER / AUTH_PASS or AUTH_MODE=oidc.',
    readOnlyNote: 'Read-only account: these settings can only be changed by an administrator.',
    err_purge_confirm: 'Type PURGE to confirm',
    liveZoomed: 'Live view is only available in Stream mode',
    connZoom: 'live paused (time range)',
    viewAria: 'Display mode', viewStream: 'Stream', viewPeriod: 'Time range',
    viewStreamTitle: 'Stream: the latest logs over a sliding duration, updated live',
    viewPeriodTitle: 'Time range: the logs between a start and an end date',
    perFrom: 'Start', perTo: 'End', perPrev: 'Previous time range', perNext: 'Next time range',
    perOut: 'Zoom out (twice as long)',
    histoTitle: 'Timeline',
    hScale: 'Scale', hLinear: 'Linear', hHeight: 'Height', hColor: 'Color',
    hColSeverity: 'Severity', hColIntensity: 'Intensity', hColNone: 'None',
    hRender: 'Display', hBars: 'Bars', hArea: 'Area',
    hStep: 'Division', hAuto: 'Automatic', hRefresh: 'Refresh', hOff: 'Off', hRefreshAuto: 'At each new interval',
    histoHelp: 'Click a bar to show its interval in Time range mode, or drag across several. The √ scale keeps small volumes visible next to bursts. Intensity compares each interval with the median of the window: calm, burst (more than 3×), anomaly (more than 10×). In live mode, the last interval is updated as messages arrive.',
    hLeg_err: 'error and above', hLeg_warn: 'warning', hLeg_other: 'notice, info, debug',
    hLeg_calm: 'calm', hLeg_burst: 'burst > 3×', hLeg_anom: 'anomaly > 10×',
    hTotal: 'Total', hRatio: (x) => `${x}× the median`, hUnshown: 'not detailed (high rate)',
    hDivision: (s) => `interval ${s}`, hCapped: 'enlarged',
    hCappedTitle: 'The chosen division would exceed 300 intervals over this range: it has been enlarged.',
    hUnzoom: '× Back to stream', unitDay: 'd',
    toTop: 'Back to top',
    colRcv: 'Received', colMt: 'Message time', colSev: 'Severity', colHost: 'Host', colApp: 'App', colCodes: 'Pattern', colMsg: 'Message',
    codesTitle: 'Codes of the color tags found in the message',
    tagCodeTitle: 'Code shown on the log lines this tag matches',
    colGrip: 'Drag to resize, double-click for the automatic width',
    resetCols: 'Reset column widths', colsReset: 'Column widths reset',
    colsHelp: 'Drag the edge of a column header in the log list to resize it (remembered by this browser).',
  },
  fr: {
    locale: 'fr-FR',
    searchAria: 'Rechercher dans les logs',
    searchSimple: 'Rechercher…  error -timeout "disk full"   (touche /)',
    searchLogsql: 'LogsQL : error AND host:web-01 | sort by (_time)',
    modeTitle: 'Basculer entre recherche simple et LogsQL',
    live: 'Direct',
    liveTitle: 'Afficher les nouveaux messages en direct',
    liveUnavailable: 'Le direct n\'est pas disponible en mode LogsQL',
    settings: 'Paramètres',
    theme: 'Thème clair / sombre',
    logout: 'Se déconnecter',
    close: 'Fermer',
    rangeAria: 'Période', severityAria: 'Sévérité', hostAria: 'Hôte', appAria: 'Application',
    histoAria: 'Volume de logs dans le temps',
    r5m: '5 min', r15m: '15 min', r1h: '1 heure', r6h: '6 heures', r24h: '24 heures', r7d: '7 jours', r30d: '30 jours', rAll: 'Tout',
    sevAll: 'Toutes sévérités', sev2: 'Critiques et +', sev3: 'Erreurs et +',
    sev4: 'Warnings et +', sev5: 'Notice et +', sev6: 'Info et +',
    allHosts: 'Tous les hôtes', allApps: 'Toutes les applis',
    emptyTitle: 'Aucun log ne correspond.',
    emptyHint: 'Élargissez la période ou envoyez des messages syslog sur le port 514 (UDP ou TCP).',
    count: (n) => `${n} ${n > 1 ? 'logs affichés' : 'log affiché'}`,
    histoMax: (n) => `max ${n} / intervalle`,
    searchError: 'Erreur de recherche : ',
    connLive: 'direct', connConnecting: 'connexion…', connRetry: 'reconnexion…',
    connPaused: 'direct en pause', connLogsql: 'direct indisponible (LogsQL)',
    newLogs: (n) => `↑ ${n} ${n > 1 ? 'nouveaux logs' : 'nouveau log'}`,
    skipped: (n) => `${n} messages non affichés (débit trop élevé)`,
    stats: (s) => `reçus ${s.received} · stockés ${s.ingested} · perdus ${s.dropped} · file ${s.queue}`,
    storageErr: 'stockage : ',
    spooled: (n) => `${n} en attente sur disque`,
    unreachable: 'serveur injoignable',
    fTime: 'horodatage', fMsg: 'message', fPid: 'pid', fSd: 'données structurées',
    filterHost: 'Filtrer sur cet hôte', filterApp: 'Filtrer sur cette appli',
    copyMsg: 'Copier le message', copyJson: 'Copier en JSON', copied: 'Copié',
    language: 'Langue',
    tagsTitle: 'Tags de couleur',
    tagsHelp: 'Les mots-clés sont surlignés dans les messages. Les modifications sont enregistrées automatiquement. En cas de chevauchement, le tag le plus haut dans la liste l\'emporte.',
    addTag: '+ Ajouter un tag', resetTags: 'Rétablir les tags par défaut',
    optWhole: 'mot entier', optWholeTitle: 'Ne colorer que les mots entiers (évite « ok » dans « token »)',
    optCaseTitle: 'Respecter la casse', optRegexTitle: 'Expression régulière',
    optActive: 'actif', optActiveTitle: 'Activer ce tag',
    colorAria: 'Couleur', keyword: 'mot-clé', preview: 'aperçu', del: 'Supprimer', noTags: 'Aucun tag.',
    newTag: 'nouveau',
    confirmDelete: (p) => `Supprimer le tag « ${p} » ?`,
    confirmReset: 'Remplacer tous les tags par les tags par défaut (warning, error) ?',
    presetAria: 'Ajouter un préréglage', presetPick: '+ Préréglage…',
    tagFilter: 'Filtrer les tags (mot-clé, libellé ou code)',
    tagCount: (n) => (n.shown === n.total ? `${n.total} tag${n.total > 1 ? 's' : ''}` : `${n.shown} / ${n.total} tags`),
    noTagMatch: 'Aucun tag ne correspond au filtre.',
    presetAdded: (n) => (n ? `${n} tag(s) ajouté(s)` : 'Ces tags sont déjà dans la liste'),
    presetsFileErr: (f) => `Fichier de préréglages ${f} invalide, préréglages intégrés utilisés : `,
    tagsLoadErr: 'Tags : ',
    err_pattern_required: 'Le mot-clé est obligatoire',
    err_invalid_color: 'Couleur invalide (format #rrggbb attendu)',
    err_invalid_regex: 'Expression régulière invalide',
    err_tag_not_found: 'Tag introuvable',
    err_too_many_tags: 'Trop de tags (99 au maximum)',
    err_live_logsql: 'Le direct n\'est pas disponible en mode LogsQL',
    dateTime: 'Date et heure',
    tzLabel: 'Fuseau horaire',
    fmtLabel: 'Format de l\'heure de réception',
    tzBrowser: (z) => `Fuseau du navigateur (${z})`,
    tzGroups: { General: 'Général', Europe: 'Europe', Africa: 'Afrique', America: 'Amériques', Asia: 'Asie', Oceania: 'Océanie et Pacifique', Other: 'Atlantique et océan Indien' },
    fmtEpoch: 'Epoch Unix (millisecondes)',
    timePreview: 'Aperçu : ',
    timeHelp: 'L\'heure de réception provient de l\'horloge du serveur. Le fuseau horaire s\'applique à toutes les dates affichées.',
    fReceived: 'réception',
    rcvTitle: 'Reçu par le serveur',
    msgTimeTitle: 'Horodatage contenu dans le message',
    fHostName: 'nom d\'hôte (DNS)', fHostIp: 'IP de l\'hôte',
    clickHost: 'Cliquer pour filtrer sur cet hôte', clickApp: 'Cliquer pour filtrer sur cette appli',
    sourceAria: 'Source', srcAll: 'Toutes les sources', tabSources: 'Sources de logs',
    srcHost: 'Système hôte',
    srcKernel: 'Noyau (dmesg)',
    auditTitle: 'Audit Linux (auditd)',
    auditEnabled: 'Collecter les événements d\'audit de cette machine',
    auditPath: 'Fichier',
    auditOff: 'Désactivé : le journal d\'audit de la machine qui héberge LogStream n\'est pas lu.',
    auditOk: ({ p, r }) => `Suivi de ${p} : ${r} événements depuis le démarrage.`,
    audit_not_found: 'Journal d\'audit introuvable : montez /var/log/audit dans le conteneur (voir docker-compose.yml) et vérifiez le chemin. ',
    audit_permission: 'Accès refusé au journal d\'audit : réglez log_group dans /etc/audit/auditd.conf, ou ajoutez une ACL pour l\'UID 10001 (voir le README). ',
    audit_read: 'Lecture du journal d\'audit impossible : ',
    auditHelp: 'Lit le journal du démon d\'audit Linux (auditd) : connexions, sudo, fichiers surveillés, appels système visés par vos règles… Les enregistrements d\'un même événement sont regroupés sur une ligne, avec un résumé (qui, quoi, résultat) et l\'enregistrement brut en dessous ; les échecs sont des warnings. À l\'activation, la dernière heure est lue d\'abord (AUDIT_BACKFILL) ; les rotations sont suivies et la lecture reprend où elle s\'était arrêtée.',
    selfTitle: 'Logs internes de LogStream',
    selfEnabled: 'Enregistrer les événements de LogStream lui-même',
    selfLevel: 'Niveau minimal',
    selfOff: 'Désactivé : les événements de LogStream ne vont que dans la sortie de son conteneur.',
    selfOk: ({ n }) => `${n} événements enregistrés depuis le démarrage (hôte logstream).`,
    selfDropped: (n) => ` ${n} écartés (répétés ou trop nombreux d\'un coup).`,
    selfHelp: 'Transforme les événements de LogStream en logs (hôte logstream, app = composant) : VictoriaLogs injoignable puis de retour, lots perdus, erreurs du listener syslog, flux Docker décrochés, sources en échec, démarrage et arrêt. Un même événement est enregistré au plus une fois toutes les 30 s, avec le nombre de répétitions.',
    hostTitle: 'Logs système de l\'hôte',
    hostEnabled: 'Collecter les logs système de cette machine',
    hostOff: 'Désactivé : les logs système de la machine qui héberge LogStream ne sont pas collectés.',
    hostWaiting: 'Démarrage…',
    hostJournal: ({ n, r }) => `Lecture du journal systemd (${n} fichiers) : ${r} entrées depuis le démarrage.`,
    hostFiles: ({ n, r }) => `Suivi de ${n} fichiers de /var/log : ${r} lignes depuis le démarrage.`,
    hostCompressed: (n) => ` ${n} champs compressés ignorés (messages longs, lisibles avec journalctl).`,
    host_not_mounted: 'Aucun répertoire de logs de l\'hôte trouvé : montez /var/log et /run/log/journal sous /host (voir docker-compose.yml).',
    host_permission: 'Accès refusé : donnez au conteneur un groupe autorisé à lire les logs (HOST_LOGS_GID dans .env, voir le README). ',
    host_empty: 'Ni journal systemd ni fichier de log trouvé dans /var/log.',
    host_read: 'Lecture des logs de l\'hôte impossible : ',
    hostHelp: 'Lit le journal systemd de l\'hôte (/var/log/journal, /run/log/journal), ou les fichiers texte de /var/log (syslog, messages, *.log) s\'il n\'y a pas de journal. Les répertoires sont montés en lecture seule. À l\'activation, la dernière heure est lue d\'abord (HOST_LOGS_BACKFILL).',
    dmesgTitle: 'Messages du noyau (dmesg)',
    dmesgEnabled: 'Collecter les messages du noyau de cette machine',
    dmesgOff: 'Désactivé : les messages du noyau de la machine qui héberge LogStream ne sont pas collectés.',
    dmesgOk: ({ d, r }) => `Lecture de ${d} : ${r} messages depuis le démarrage.`,
    dmesgLost: (n) => ` ${n} messages ont été écrasés dans le buffer du noyau avant d'être lus.`,
    dmesg_not_found: 'Périphérique /dev/kmsg absent du conteneur : décommentez sa section devices dans docker-compose.yml (voir le README).',
    dmesg_permission: 'Accès refusé à /dev/kmsg : réglez kernel.dmesg_restrict=0 sur l\'hôte, ou lancez le conteneur en root avec la capacité SYSLOG (voir le README). ',
    dmesg_read: 'Lecture des messages du noyau impossible : ',
    dmesgHelp: 'Lit le buffer circulaire du noyau de l\'hôte via /dev/kmsg, comme dmesg : matériel, pilotes, OOM killer, pare-feu… À l\'activation, la dernière heure est lue d\'abord (DMESG_BACKFILL) ; après un redémarrage, la lecture reprend où elle s\'était arrêtée.',
    syslogEnabled: 'Recevoir les messages syslog', syslogProtocols: 'Protocoles', syslogPort: 'Port',
    syslogListening: ({ port, protos }) => `Écoute sur le port ${port} (${protos}).`,
    syslogOff: 'Réception syslog désactivée : les messages syslog sont ignorés (les logs Docker sont toujours collectés).',
    syslogNone: 'Aucun protocole sélectionné : les messages syslog sont ignorés.',
    syslogErr: (p) => `Impossible d'écouter en ${p} : `,
    syslogPortHelp: (i) => `Publié par docker-compose (port ${i} dans le conteneur). Pour le changer, modifiez SYSLOG_PORT dans .env, puis lancez docker compose up -d.`,
    dockerTitle: 'Conteneurs Docker',
    dockerDefault: 'Suivre automatiquement les nouveaux conteneurs',
    dockerAll: 'Tout activer', dockerNone: 'Tout désactiver',
    dockerOff: 'La source Docker est désactivée sur ce serveur. Réglez DOCKER_LOGS=on (voir docker-compose.yml).',
    dockerErr: 'Docker injoignable : ',
    dockerOk: ({ h, n, f }) => `Connecté à Docker sur ${h} : ${n} conteneurs, ${f} suivis.`,
    dockerStopped: (n) => `Afficher les conteneurs arrêtés (${n})`,
    dockerFilter: 'Filtrer les conteneurs',
    dockerLegend: 'En couleur : suivis · en gris : non suivis · pointillés : arrêtés · la couleur identifie le projet compose. Cliquez sur une étiquette pour la basculer.',
    dockerNoMatch: 'Aucun conteneur',
    stFollowing: 'suivi', stSelected: 'suivi dès qu\'il tourne', stIgnored: 'non suivi', stStopped: 'arrêté',
    stLocked: 'exclu', stLockedTitle: 'LogStream lui-même, ou étiquette logstream.exclude=true',
    dockerHelp: 'Les logs sont lus via docker-socket-proxy, une passerelle en lecture seule : LogStream peut lister les conteneurs et lire leurs logs, rien d\'autre. Les choix s\'appliquent par service compose (ou nom de conteneur), ils survivent donc à la recréation des conteneurs.',
    fContainer: 'conteneur', fContainerId: 'ID du conteneur', fImage: 'image', fProject: 'projet compose',
    fService: 'service compose', fStream: 'flux', fSourceType: 'source',
    fUnit: 'unité systemd', fLogFile: 'fichier de log',
    export: 'Exporter', exportCsvHint: 'Séparateur virgule, UTF-8',
    exportExcel: 'CSV pour Excel', exportExcelHint: 'Séparateur point-virgule, pour Excel en français',
    exportNote: (n) => `Jusqu'à ${n} logs correspondant aux filtres, du plus récent au plus ancien. Dates dans le fuseau choisi dans Paramètres.`,
    exporting: 'Export…', exportDone: (f) => `Téléchargé : ${f}`, exportError: 'Échec de l\'export : ',
    err_export_pipes: 'L\'export ne gère pas les pipes LogsQL (| …) : gardez seulement la partie filtre',
    tabLocale: 'Localisation et langue', tabFilters: 'Motifs de recherche', tabInterface: 'Affichage', tabData: 'Base de données',
    themeTitle: 'Thème', themeSystem: 'Système', themeLight: 'Clair', themeDark: 'Sombre',
    themeHelp: 'Système suit la préférence clair/sombre de votre ordinateur ou de votre téléphone.',
    logDisplay: 'Affichage des logs', fontSize: 'Taille du texte',
    sizeTiny: 'Très petite', sizeSmall: 'Petite', sizeMedium: 'Moyenne', sizeLarge: 'Grande',
    density: 'Densité', densityNormal: 'Normale', densityCompact: 'Compacte',
    fontLabel: 'Police', fontSystem: 'Monospace du système (aucun téléchargement)', fontBuiltin: 'intégrée, étroite',
    fontHelp: 'Polices libres. Les polices intégrées (Inconsolata Condensed, la plus étroite, Iosevka et Ubuntu Mono) sont servies par LogStream lui-même et fonctionnent hors ligne ; elles sont étroites, donc chaque ligne affiche plus de texte. Les autres sont chargées par votre navigateur depuis Bunny Fonts, un service européen respectueux de la vie privée ; sans accès à internet, la police du système est utilisée.',
    previewMsgs: ['error: link down on eth1, carrier lost', 'warning: disk /dev/sda temperature 58°C', 'backup finished: ok (12.4 GB in 00:14:32)'],
    dangerZone: 'Zone de danger',
    dbTitle: 'Base de données', dbRefresh: 'Actualiser', dbLoading: 'Chargement…',
    dbUnits: ['o', 'Ko', 'Mo', 'Go', 'To'],
    dbRows: 'Lignes stockées', dbDisk: 'Taille sur disque', dbIndex: (x) => `dont ${x} d'index`,
    dbRaw: 'Taille brute', dbRatio: (x) => `compression ×${x}`,
    dbPeriod: 'Période', dbFromTo: (p) => `${p.from} → ${p.to}`, dbDays: (n) => `${n} jour${n > 1 ? 's' : ''}`,
    dbRetention: (r) => `rétention ${r}`,
    dbRecent: 'Dernières 24 h', dbLastHour: (n) => `${n} sur la dernière heure`,
    dbSources: 'Sources', dbHostsApps: (p) => `${p.h} hôte${p.hn > 1 ? 's' : ''} · ${p.a} application${p.an > 1 ? 's' : ''}`,
    dbFree: 'Espace disque libre', dbEmpty: 'La base est vide.',
    dbQueryErr: 'Certains chiffres n\'ont pas pu être calculés : ',
    purgeHelp: 'Supprime définitivement tous les logs stockés pour repartir de zéro. Les tags de couleur et les réglages sont conservés.',
    purgeBtn: 'Supprimer tous les logs…',
    purgePrompt: 'Cette action supprime définitivement TOUS les logs stockés.\n\nTapez PURGE pour confirmer :',
    purgeStarted: 'Suppression lancée',
    purgeRunning: 'Suppression en cours… les logs disparaissent progressivement.',
    purgeDone: 'Tous les logs ont été supprimés.',
    purgeForbidden: 'La purge est désactivée sur ce serveur (ALLOW_PURGE=false).',
    err_purge_unavailable: 'VictoriaLogs refuse les suppressions : lancez-le avec -delete.enable (voir docker-compose.yml)',
    err_purge_forbidden: 'La purge est désactivée sur ce serveur (ALLOW_PURGE=false)',
    err_read_only: 'Compte en lecture seule : les modifications sont réservées aux administrateurs',
    err_cross_site: 'Requête refusée : elle vient d\'un autre site',
    authOff: 'Aucune authentification : toute personne qui atteint cette page peut lire les logs et changer les réglages. Définissez AUTH_USER / AUTH_PASS ou AUTH_MODE=oidc.',
    readOnlyNote: 'Compte en lecture seule : seul un administrateur peut modifier ces réglages.',
    err_purge_confirm: 'Tapez PURGE pour confirmer',
    liveZoomed: 'Le direct n\'est disponible qu\'en mode Flux',
    connZoom: 'direct en pause (période)',
    viewAria: 'Mode d\'affichage', viewStream: 'Flux', viewPeriod: 'Période',
    viewStreamTitle: 'Flux : les derniers logs sur une durée glissante, mis à jour en direct',
    viewPeriodTitle: 'Période : les logs entre une date de début et une date de fin',
    perFrom: 'Début', perTo: 'Fin', perPrev: 'Période précédente', perNext: 'Période suivante',
    perOut: 'Élargir (deux fois plus long)',
    histoTitle: 'Frise',
    hScale: 'Échelle', hLinear: 'Linéaire', hHeight: 'Hauteur', hColor: 'Couleur',
    hColSeverity: 'Sévérité', hColIntensity: 'Intensité', hColNone: 'Aucune',
    hRender: 'Rendu', hBars: 'Barres', hArea: 'Aire',
    hStep: 'Division', hAuto: 'Automatique', hRefresh: 'Rafraîchissement', hOff: 'Désactivé', hRefreshAuto: 'À chaque nouvel intervalle',
    histoHelp: 'Cliquez sur une barre pour afficher son intervalle en mode Période, ou glissez sur plusieurs. L\'échelle √ garde visibles les petits volumes à côté des rafales. L\'intensité compare chaque intervalle à la médiane de la fenêtre : calme, rafale (plus de 3×), anomalie (plus de 10×). En direct, le dernier intervalle se met à jour à l\'arrivée des messages.',
    hLeg_err: 'error et plus', hLeg_warn: 'warning', hLeg_other: 'notice, info, debug',
    hLeg_calm: 'calme', hLeg_burst: 'rafale > 3×', hLeg_anom: 'anomalie > 10×',
    hTotal: 'Total', hRatio: (x) => `${x}× la médiane`, hUnshown: 'sans détail (débit élevé)',
    hDivision: (s) => `intervalle ${s}`, hCapped: 'élargi',
    hCappedTitle: 'La division choisie dépasserait 300 intervalles sur cette plage : elle a été élargie.',
    hUnzoom: '× Revenir au flux', unitDay: 'j',
    toTop: 'Revenir en haut',
    colRcv: 'Réception', colMt: 'Heure message', colSev: 'Sévérité', colHost: 'Hôte', colApp: 'App', colCodes: 'Motif de recherche', colMsg: 'Message',
    codesTitle: 'Codes des tags de couleur trouvés dans le message',
    tagCodeTitle: 'Code affiché sur les lignes de log où ce tag est trouvé',
    colGrip: 'Glisser pour redimensionner, double-clic pour la largeur automatique',
    resetCols: 'Réinitialiser les colonnes', colsReset: 'Largeurs de colonnes réinitialisées',
    colsHelp: 'Glissez le bord d\'un en-tête de colonne de la liste pour la redimensionner (mémorisé par ce navigateur).',
  },
};

/* ================= Utilities ================= */

const $ = (sel) => document.querySelector(sel);
const PAGE_SIZE = 500;   // rows loaded per search
const MAX_ROWS = 2000;   // rows kept on screen in live mode
const RANGE_MS = { '5m': 3e5, '15m': 9e5, '1h': 36e5, '6h': 216e5, '24h': 864e5, '7d': 6048e5, '30d': 2592e6 };

const store = {
  get(k, def) { try { const v = localStorage.getItem('logstream.' + k); return v === null ? def : v; } catch { return def; } },
  set(k, v) { try { localStorage.setItem('logstream.' + k, v); } catch { /* storage unavailable */ } },
};

let lang = I18N[store.get('lang', 'en')] ? store.get('lang', 'en') : 'en';
function t(key, arg) {
  const v = I18N[lang][key] ?? I18N.en[key] ?? key;
  return typeof v === 'function' ? v(arg) : v;
}

const ESC = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ESC[c]);
const pad = (n, w = 2) => String(n).padStart(w, '0');
const debounce = (fn, ms) => { let tm; return (...a) => { clearTimeout(tm); tm = setTimeout(() => fn(...a), ms); }; };
const fmtNum = (n) => Number(n || 0).toLocaleString(t('locale'));

// Date.parse only accepts 3 decimals everywhere: drop the nanoseconds.
const parseTime = (s) => new Date(String(s || '').replace(/(\.\d{3})\d+/, '$1'));

/* ================= Time zones and time formats ================= */

const TIME_ZONES = {
  General: ['UTC'],
  Europe: ['Europe/London', 'Europe/Dublin', 'Europe/Lisbon', 'Europe/Paris', 'Europe/Brussels', 'Europe/Amsterdam',
    'Europe/Luxembourg', 'Europe/Berlin', 'Europe/Zurich', 'Europe/Madrid', 'Europe/Rome', 'Europe/Vienna',
    'Europe/Prague', 'Europe/Warsaw', 'Europe/Stockholm', 'Europe/Oslo', 'Europe/Copenhagen', 'Europe/Helsinki',
    'Europe/Athens', 'Europe/Bucharest', 'Europe/Kyiv', 'Europe/Istanbul', 'Europe/Moscow'],
  Africa: ['Africa/Casablanca', 'Africa/Algiers', 'Africa/Tunis', 'Africa/Dakar', 'Africa/Abidjan', 'Africa/Lagos',
    'Africa/Cairo', 'Africa/Nairobi', 'Africa/Johannesburg'],
  America: ['America/St_Johns', 'America/Halifax', 'America/Montreal', 'America/Toronto', 'America/New_York',
    'America/Chicago', 'America/Denver', 'America/Phoenix', 'America/Los_Angeles', 'America/Vancouver',
    'America/Anchorage', 'America/Mexico_City', 'America/Martinique', 'America/Guadeloupe', 'America/Cayenne',
    'America/Bogota', 'America/Lima', 'America/Santiago', 'America/Sao_Paulo', 'America/Buenos_Aires'],
  Asia: ['Asia/Dubai', 'Asia/Tehran', 'Asia/Karachi', 'Asia/Kolkata', 'Asia/Kathmandu', 'Asia/Dhaka', 'Asia/Bangkok',
    'Asia/Ho_Chi_Minh', 'Asia/Jakarta', 'Asia/Singapore', 'Asia/Hong_Kong', 'Asia/Shanghai', 'Asia/Taipei',
    'Asia/Manila', 'Asia/Seoul', 'Asia/Tokyo'],
  Oceania: ['Australia/Perth', 'Australia/Adelaide', 'Australia/Brisbane', 'Australia/Sydney', 'Pacific/Noumea',
    'Pacific/Auckland', 'Pacific/Tahiti', 'Pacific/Honolulu'],
  Other: ['Atlantic/Reykjavik', 'Atlantic/Azores', 'Atlantic/Canary', 'Indian/Reunion', 'Indian/Mayotte', 'Indian/Mauritius'],
};

// Each format receives the date parts in the selected time zone (and the Date itself).
const TIME_FORMATS = [
  { id: 'dmy', label: 'DD/MM/YYYY HH:MM:SS', f: (p) => `${p.D}/${p.M}/${p.Y} ${p.h}:${p.m}:${p.s}` },
  { id: 'dmy-ms', label: 'DD/MM/YYYY HH:MM:SS.mmm', f: (p) => `${p.D}/${p.M}/${p.Y} ${p.h}:${p.m}:${p.s}.${p.ms}` },
  { id: 'ymd', label: 'YYYY-MM-DD HH:MM:SS', f: (p) => `${p.Y}-${p.M}-${p.D} ${p.h}:${p.m}:${p.s}` },
  { id: 'ymd-ms', label: 'YYYY-MM-DD HH:MM:SS.mmm', f: (p) => `${p.Y}-${p.M}-${p.D} ${p.h}:${p.m}:${p.s}.${p.ms}` },
  { id: 'mdy12', label: 'MM/DD/YYYY hh:MM:SS.mmm AM/PM', f: (p) => {
    const H = Number(p.h);
    return `${p.M}/${p.D}/${p.Y} ${pad(H % 12 || 12)}:${p.m}:${p.s}.${p.ms} ${H < 12 ? 'AM' : 'PM'}`;
  } },
  { id: 'time-ms', label: 'HH:MM:SS.mmm', f: (p) => `${p.h}:${p.m}:${p.s}.${p.ms}` },
  { id: 'iso', label: 'ISO 8601 (YYYY-MM-DDTHH:MM:SS.mmm+hh:mm)', f: (p) => `${p.Y}-${p.M}-${p.D}T${p.h}:${p.m}:${p.s}.${p.ms}${p.off === '+00:00' ? 'Z' : p.off}` },
  { id: 'epoch', label: null, f: (p, d) => String(d.getTime()) },
];

const DEFAULT_TIME_FMT = 'dmy'; // usual French display: 28/09/2026 14:55:23
const timePrefs = { tz: '', fmt: DEFAULT_TIME_FMT }; // tz '' = browser time zone
const partsFormatters = new Map();

function validTimeZone(tz) {
  if (!tz) return true;
  try { new Intl.DateTimeFormat('en-US', { timeZone: tz }); return true; } catch { return false; }
}

// Date parts in a time zone ('' = browser zone), plus the UTC offset as "+hh:mm".
function zoneParts(date, tz = timePrefs.tz) {
  let f = partsFormatters.get(tz);
  if (!f) {
    f = new Intl.DateTimeFormat('en-US', {
      timeZone: tz || undefined, year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23', timeZoneName: 'longOffset',
    });
    partsFormatters.set(tz, f);
  }
  const p = {};
  for (const x of f.formatToParts(date)) p[x.type] = x.value;
  const off = (p.timeZoneName || 'GMT').replace('GMT', '') || '+00:00';
  return {
    Y: p.year, M: p.month, D: p.day, h: p.hour === '24' ? '00' : p.hour, m: p.minute, s: p.second,
    ms: pad(date.getUTCMilliseconds(), 3), off,
  };
}

function fmtWith(fmtId, date) {
  const fmt = TIME_FORMATS.find((x) => x.id === fmtId) || TIME_FORMATS[0];
  return fmt.f(zoneParts(date), date);
}

// Full date in the selected format (reception time, details panel).
function fmtFull(s) {
  const d = parseTime(s);
  return isNaN(d) ? String(s ?? '') : fmtWith(timePrefs.fmt, d);
}

async function api(url, opts = {}) {
  if (opts.body && typeof opts.body !== 'string') {
    opts.body = JSON.stringify(opts.body);
    opts.headers = { 'Content-Type': 'application/json' };
  }
  const res = await fetch(url, opts);
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { /* not JSON */ }
  if (!res.ok) throw apiError(res, text, data);
  return data;
}

// Known error codes are translated; otherwise the server message is shown.
function apiError(res, text, data) {
  // OIDC session expired: reloading the page goes through the login again.
  if (res.status === 401 && data && data.code === 'auth') location.reload();
  let msg = (data && data.error) || text || res.statusText;
  if (data && data.code && I18N[lang]['err_' + data.code]) {
    msg = t('err_' + data.code) + (data.detail ? (lang === 'fr' ? ' : ' : ': ') + data.detail : '');
  }
  return new Error(msg);
}

function toast(msg) {
  const el = $('#toast');
  // A modal dialog sits above everything: move the toast into it so it stays visible.
  const dlg = $('#settingsDlg');
  (dlg.open ? dlg : document.body).append(el);
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toast.tm);
  toast.tm = setTimeout(() => { el.hidden = true; }, 1800);
}

/* ================= State ================= */

const state = {
  rows: [],          // displayed records, newest first
  pending: [],       // live messages received while scrolled down
  tags: [],
  presets: [],       // preset groups from /api/presets
  matchers: [],      // compiled tags + search terms
  mode: store.get('mode', 'simple'),
  live: store.get('live', '1') === '1',
  es: null,
  reqId: 0,
  zoom: null,        // {from, to} in ms in Time range mode (range "custom"), null in Stream mode
  tookMs: null,
  conn: ['', 'connPaused'],
  stats: null,
};
const recOf = new WeakMap(); // .row element -> record

const list = $('#list');

/* ================= Language ================= */

function applyLang() {
  document.documentElement.lang = lang;
  for (const el of document.querySelectorAll('[data-i18n]')) el.textContent = t(el.dataset.i18n);
  for (const el of document.querySelectorAll('[data-i18n-title]')) el.title = t(el.dataset.i18nTitle) + (el.dataset.user ? ` (${el.dataset.user})` : '');
  for (const el of document.querySelectorAll('[data-i18n-aria]')) el.setAttribute('aria-label', t(el.dataset.i18nAria));
  for (const el of document.querySelectorAll('[data-i18n-ph]')) el.placeholder = t(el.dataset.i18nPh);
  for (const b of document.querySelectorAll('#langSwitch [data-lang]')) b.setAttribute('aria-checked', String(b.dataset.lang === lang));
}

function setLang(next) {
  if (next === lang || !I18N[next]) return;
  lang = next;
  store.set('lang', lang);
  applyLang();
  syncControls();
  setConn(...state.conn);
  renderList();
  updateCount(state.tookMs);
  renderPeriod();
  renderHisto();
  renderStats();
  renderTagList();
  renderPresets();
  renderTimeSettings();
  renderPurge();
  renderDbStats();
  renderInterface();
  renderSyslog();
  renderHost();
  renderDmesg();
  renderAudit();
  renderSelfLog();
  renderDocker();
}

$('#langSwitch').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-lang]');
  if (b) setLang(b.dataset.lang);
});

/* ================= Theme ================= */

function effectiveTheme() {
  const th = document.documentElement.dataset.theme;
  if (th) return th;
  return matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}
// mode: 'system' (follow the OS), 'light' or 'dark'.
function setTheme(mode) {
  if (mode === 'light' || mode === 'dark') document.documentElement.dataset.theme = mode;
  else { delete document.documentElement.dataset.theme; mode = 'system'; }
  store.set('theme', mode);
  renderThemeSwitch();
}

function renderThemeSwitch() {
  const mode = document.documentElement.dataset.theme || 'system';
  for (const b of document.querySelectorAll('#themeSwitch [data-theme-opt]')) {
    b.setAttribute('aria-checked', String(b.dataset.themeOpt === mode));
  }
}

// The header button switches between light and dark.
$('#themeBtn').addEventListener('click', () => setTheme(effectiveTheme() === 'dark' ? 'light' : 'dark'));
$('#themeSwitch').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-theme-opt]');
  if (b) setTheme(b.dataset.themeOpt);
});

/* ================= Log font and size ================= */

// Free monospace fonts, served by Bunny Fonts (https://fonts.bunny.net), except the
// built-in ones (web/fonts, declared in style.css), which also work offline.
const LOG_FONTS = [
  { id: 'system' },
  { id: 'inconsolata-condensed', family: 'Inconsolata Condensed', builtin: true },
  { id: 'iosevka', family: 'Iosevka', builtin: true },
  { id: 'ubuntu-mono', family: 'Ubuntu Mono', builtin: true },
  { id: 'jetbrains-mono', family: 'JetBrains Mono' },
  { id: 'fira-code', family: 'Fira Code' },
  { id: 'source-code-pro', family: 'Source Code Pro' },
  { id: 'ibm-plex-mono', family: 'IBM Plex Mono' },
  { id: 'cascadia-code', family: 'Cascadia Code' },
  { id: 'roboto-mono', family: 'Roboto Mono' },
  { id: 'inconsolata', family: 'Inconsolata' },
  { id: 'red-hat-mono', family: 'Red Hat Mono' },
  { id: 'noto-sans-mono', family: 'Noto Sans Mono' },
  { id: 'victor-mono', family: 'Victor Mono' },
  { id: 'dm-mono', family: 'DM Mono', weights: '400,500' },
];
const LOG_SIZES = { tiny: '10.5px', small: '11.5px', medium: '12.5px', large: '14px' };
const LOG_DENSITIES = ['normal', 'compact'];
const ui = { font: 'system', size: 'medium', density: 'normal' };

function applyLogFont(id) {
  const f = LOG_FONTS.find((x) => x.id === id) || LOG_FONTS[0];
  ui.font = f.id;
  const root = document.documentElement.style;
  if (!f.family) {
    root.removeProperty('--log-font');
    return;
  }
  if (f.builtin) {
    root.setProperty('--log-font', `'${f.family}', var(--mono)`);
    return;
  }
  const href = `https://fonts.bunny.net/css?family=${f.id}:${f.weights || '400,700'}&display=swap`;
  let link = document.getElementById('logFontCss');
  if (!link) {
    link = Object.assign(document.createElement('link'), { id: 'logFontCss', rel: 'stylesheet' });
    document.head.append(link);
  }
  if (link.getAttribute('href') !== href) link.href = href;
  root.setProperty('--log-font', `'${f.family}', var(--mono)`);
}

function applyLogSize(id) {
  ui.size = LOG_SIZES[id] ? id : 'medium';
  document.documentElement.style.setProperty('--log-size', LOG_SIZES[ui.size]);
}

function applyLogDensity(id) {
  ui.density = LOG_DENSITIES.includes(id) ? id : 'normal';
  document.documentElement.dataset.density = ui.density;
}

function renderInterface() {
  renderThemeSwitch();
  for (const b of document.querySelectorAll('#sizeSwitch [data-size]')) {
    b.setAttribute('aria-checked', String(b.dataset.size === ui.size));
  }
  for (const b of document.querySelectorAll('#densitySwitch [data-density]')) {
    b.setAttribute('aria-checked', String(b.dataset.density === ui.density));
  }
  const sel = $('#fontSelect');
  const fontName = (f) => (!f.family ? t('fontSystem') : f.builtin ? `${f.family} (${t('fontBuiltin')})` : f.family);
  sel.innerHTML = LOG_FONTS.map((f) => `<option value="${f.id}">${esc(fontName(f))}</option>`).join('');
  sel.value = ui.font;
  renderFontPreview();
  renderHistoSettings();
}

function renderFontPreview() {
  const now = new Date().toISOString();
  const [m1, m2, m3] = t('previewMsgs');
  const rows = [
    { received: now, msg_time: now, severity: 'err', host: 'fw-edge', app: 'kernel', _msg: m1 },
    { received: now, msg_time: now, severity: 'warning', host: 'nas', app: 'smartd', _msg: m2 },
    { received: now, msg_time: now, severity: 'info', host: 'web-01', app: 'backup', _msg: m3 },
  ];
  $('#fontPreview').innerHTML = rows.map((r) => rowHTML(r, false)).join('');
}

$('#sizeSwitch').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-size]');
  if (!b) return;
  applyLogSize(b.dataset.size);
  store.set('logSize', ui.size);
  renderInterface();
});
$('#densitySwitch').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-density]');
  if (!b) return;
  applyLogDensity(b.dataset.density);
  store.set('logDensity', ui.density);
  renderInterface();
});
$('#fontSelect').addEventListener('change', (ev) => {
  applyLogFont(ev.target.value);
  store.set('logFont', ui.font);
});

/* ================= Settings tabs ================= */

function showSettingsTab(name) {
  const tabs = [...document.querySelectorAll('#setNav [data-tab]')];
  if (!tabs.some((b) => b.dataset.tab === name)) name = 'locale';
  for (const b of tabs) b.setAttribute('aria-selected', String(b.dataset.tab === name));
  for (const p of document.querySelectorAll('.set-panel')) p.hidden = p.dataset.panel !== name;
  store.set('settingsTab', name);
}
$('#setNav').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-tab]');
  if (b) showSettingsTab(b.dataset.tab);
});

/* ================= Filters ================= */

function params() {
  const p = new URLSearchParams();
  const q = $('#q').value.trim();
  if (q) p.set('q', q);
  p.set('mode', state.mode);
  if (state.zoom) {
    p.set('range', 'custom');
    p.set('from', state.zoom.from);
    p.set('to', state.zoom.to);
  } else {
    p.set('range', $('#range').value);
  }
  for (const id of ['severity', 'host', 'app', 'source']) {
    const v = $('#' + id).value;
    if (v !== '') p.set(id, v);
  }
  return p;
}

function liveAvailable() {
  return !state.zoom && !(state.mode === 'logsql' && $('#q').value.trim());
}

function syncControls() {
  const logsql = state.mode === 'logsql';
  $('#mode').textContent = logsql ? 'LogsQL' : 'Simple';
  $('#mode').classList.toggle('on', logsql);
  $('#q').placeholder = logsql ? t('searchLogsql') : t('searchSimple');
  const btn = $('#live');
  btn.disabled = !liveAvailable();
  btn.classList.toggle('on', state.live && liveAvailable());
  btn.title = liveAvailable() ? t('liveTitle') : t(state.zoom ? 'liveZoomed' : 'liveUnavailable');
  for (const id of ['severity', 'host', 'app', 'source']) $('#' + id).classList.toggle('set', $('#' + id).value !== '');
}

/* ================= Tags and highlighting ================= */

// Readable text color (black or white) for a given background.
function textOn(hex) {
  const n = parseInt(hex.slice(1), 16);
  const yiq = (((n >> 16) & 255) * 299 + ((n >> 8) & 255) * 587 + (n & 255) * 114) / 1000;
  return yiq >= 150 ? '#111827' : '#ffffff';
}
const tagStyle = (color) => `background:${color};color:${textOn(color)}`;
const escapeRe = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

// Severity badges share the colors of the "error" and "warning" tags.
function applySeverityColors() {
  const style = document.documentElement.style;
  for (const name of ['error', 'warning']) {
    const tag = state.tags.find((x) => String(x.pattern).toLowerCase() === name && /^#[0-9a-f]{6}$/i.test(x.color));
    if (tag) {
      style.setProperty(`--tag-${name}`, tag.color);
      style.setProperty(`--tag-${name}-text`, textOn(tag.color));
    } else {
      style.removeProperty(`--tag-${name}`);
      style.removeProperty(`--tag-${name}-text`);
    }
  }
}

function compileMatchers() {
  applySeverityColors();
  const out = [];
  for (const tag of state.tags) {
    if (!tag.enabled || !tag.pattern) continue;
    let src = tag.regex ? tag.pattern : escapeRe(tag.pattern);
    if (tag.wholeWord) src = `(?<![\\p{L}\\p{N}_])(?:${src})(?![\\p{L}\\p{N}_])`;
    try {
      // A regex may name a group "hl" to color only that part of the match.
      const hl = tag.regex && /\(\?P?<hl>/.test(tag.pattern);
      if (hl) src = src.replace(/\(\?P<hl>/g, '(?<hl>');
      out.push({ re: new RegExp(src, 'gu' + (hl ? 'd' : '') + (tag.caseSensitive ? '' : 'i')), html: `<mark class="tag" style="${tagStyle(tag.color)}">`,
        code: tag.code, name: tag.label || tag.pattern });
    } catch (e) {
      console.warn('Tag skipped (invalid pattern):', tag.pattern, e.message);
    }
  }
  // Simple-search terms get a subtle highlight, with the lowest priority.
  if (state.mode === 'simple') {
    for (const tok of $('#q').value.match(/-?"[^"]*"|\S+/g) || []) {
      if (tok.startsWith('-')) continue;
      const word = tok.replace(/^"|"$/g, '');
      if (word) out.push({ re: new RegExp(escapeRe(word), 'giu'), html: '<mark class="hit">' });
    }
  }
  state.matchers = out;
}

function highlight(text) {
  text = String(text ?? '');
  if (!state.matchers.length || !text) return esc(text);
  const spans = [];
  state.matchers.forEach((m, prio) => {
    m.re.lastIndex = 0;
    let x;
    while ((x = m.re.exec(text)) !== null) {
      if (x[0] === '') { m.re.lastIndex++; continue; }
      const g = x.indices?.groups?.hl;
      if (g) { if (g[1] > g[0]) spans.push({ s: g[0], e: g[1], prio, html: m.html }); continue; }
      spans.push({ s: x.index, e: x.index + x[0].length, prio, html: m.html });
    }
  });
  if (!spans.length) return esc(text);
  spans.sort((a, b) => a.s - b.s || a.prio - b.prio);
  let out = '';
  let pos = 0;
  for (const sp of spans) {
    if (sp.s < pos) continue; // overlap: first one wins
    out += esc(text.slice(pos, sp.s)) + sp.html + esc(text.slice(sp.s, sp.e)) + '</mark>';
    pos = sp.e;
  }
  return out + esc(text.slice(pos));
}

// Badges with the codes of the tags found in a message, in list order. The
// column has room for 3: beyond that, 2 badges and "+N" (all in the tooltip).
function codesHTML(text) {
  text = String(text ?? '');
  const found = [];
  for (const m of state.matchers) {
    if (!m.code || !text) continue;
    m.re.lastIndex = 0;
    let x;
    while ((x = m.re.exec(text)) !== null) {
      if (x[0] === '') { m.re.lastIndex++; continue; }
      const g = x.indices?.groups?.hl;
      if (!x.indices?.groups || g) { found.push(m); break; }
    }
  }
  if (!found.length) return '';
  const shown = found.length > 3 ? found.slice(0, 2) : found;
  const title = found.map((m) => `${m.code} ${m.name}`).join('\n');
  return `<span title="${esc(title)}">`
    + shown.map((m) => `<b>${esc(m.code)}</b>`).join('')
    + (found.length > 3 ? `<b class="more">+${found.length - 2}</b>` : '')
    + '</span>';
}

/* ================= List rendering ================= */

const SEV_CLASS = { emerg: 'crit', alert: 'crit', crit: 'crit', err: 'err', warning: 'warning', notice: 'notice', info: 'info', debug: 'debug' };

function fmtTime(s) {
  const d = parseTime(s);
  if (isNaN(d)) return esc(s);
  const p = zoneParts(d);
  const day = lang === 'fr' ? `${p.D}/${p.M}` : `${p.M}/${p.D}`;
  return `${day} ${p.h}:${p.m}:${p.s}<span class="ms">.${p.ms}</span>`;
}

// Sources with their own look in the list: a colored edge and a badge
// before the app name (the severity badge stays the same).
const SRC_BADGE = { audit: 'AUDIT', logstream: 'LS' };

function rowHTML(r, isNew) {
  const sev = r.severity || '';
  const cls = SEV_CLASS[sev] || 'info';
  const badge = SRC_BADGE[r.source_type];
  return `<article class="row sev-${cls}${badge ? ' src-' + r.source_type : ''}${isNew ? ' new' : ''}" tabindex="0">`
    + `<time class="rcv" title="${esc(t('rcvTitle'))}">${r.received ? esc(fmtFull(r.received)) : '—'}</time>`
    + `<time class="mt" title="${esc(t('msgTimeTitle'))}">${fmtTime(r.msg_time || r._time)}</time>`
    + `<span class="sev">${esc(sev || '—')}</span>`
    + (r.host
      ? `<span class="host" data-act="host" title="${esc(hostTitle(r) + ' · ' + t('clickHost'))}">${esc(r.host_name || r.host)}</span>`
      : '<span class="host"></span>')
    + (r.app
      ? `<span class="app${r.source_type === 'docker' ? ' proj' : ''}" data-act="app"${r.source_type === 'docker' ? ` style="--h:${hueOf(r.compose_project || r.container || r.app)}"` : ''} title="${esc((r.compose_project ? r.compose_project + '/' : '') + (r.container ? r.container + ' · ' : '') + r.app + ' · ' + t('clickApp'))}">${r.source_type === 'docker' ? DOCKER_ICON : ''}${badge ? `<b class="src-badge">${badge}</b>` : ''}${esc(r.app)}</span>`
      : '<span class="app"></span>')
    + `<span class="codes">${codesHTML(r._msg)}</span>`
    + `<div class="msg">${msgHTML(r)}</div>`
    + '</article>';
}

// Message cell; audit events also show their raw record (hidden in compact
// density until the row is opened).
function msgHTML(r) {
  return highlight(r._msg)
    + (r.source_type === 'audit' && r.audit_raw ? `<div class="raw">${esc(r.audit_raw)}</div>` : '');
}

// Stable hue (0-359) for a name: the compose project color, shared by the
// Sources labels and the Docker app names in the log list.
function hueOf(name) {
  let h = 0;
  for (const ch of String(name)) h = (h * 31 + ch.codePointAt(0)) >>> 0;
  return h % 360;
}

const DOCKER_ICON = '<svg class="src-ico" viewBox="0 0 24 24" aria-hidden="true"><path d="M21 8 12 3 3 8v8l9 5 9-5z"/><path d="m3 8 9 5 9-5M12 13v8"/></svg>';

// Tooltip of the host cell: name and IP when both are known.
function hostTitle(r) {
  const ip = r.host_ip || (r.host_name ? r.host : '');
  const name = r.host_name || r.host;
  return ip ? `${name} (${ip})` : name;
}

// Map the .row elements starting at `first` to their records.
function bindRows(first, rows) {
  let el = first;
  for (const r of rows) {
    if (!el) break;
    recOf.set(el, r);
    el = el.nextElementSibling;
  }
}

// Adds the hosts/apps of displayed logs to the filter lists (live logs can
// show a host before the next refresh of /api/facets).
function addFacetValues(rows) {
  for (const id of ['host', 'app']) {
    const sel = $('#' + id);
    const have = new Set([...sel.options].map((o) => o.value));
    let added = false;
    for (const r of rows) {
      const v = r[id];
      if (!v || have.has(v)) continue;
      have.add(v);
      sel.add(new Option(id === 'host' && r.host_name ? `${r.host_name} (${v})` : v, v));
      added = true;
    }
    if (added) {
      const cur = sel.value;
      const [first, ...rest] = [...sel.options];
      rest.sort((a, b) => a.text.localeCompare(b.text));
      sel.replaceChildren(first, ...rest);
      sel.value = cur;
    }
  }
}

function renderList() {
  addFacetValues(state.rows);
  list.innerHTML = state.rows.map((r) => rowHTML(r, false)).join('');
  bindRows(list.firstElementChild, state.rows);
  $('#table').hidden = !state.rows.length;
  $('#empty').hidden = state.rows.length > 0 || !$('#error').hidden;
}

function rehighlight() {
  for (const row of list.children) {
    const r = recOf.get(row);
    if (!r) continue;
    row.querySelector('.msg').innerHTML = msgHTML(r);
    row.querySelector('.codes').innerHTML = codesHTML(r._msg);
  }
}

function prependRows(rows) {
  if (!rows.length) return;
  addFacetValues(rows);
  list.insertAdjacentHTML('afterbegin', rows.map((r) => rowHTML(r, true)).join(''));
  bindRows(list.firstElementChild, rows);
  state.rows = rows.concat(state.rows);
  $('#table').hidden = false;
  while (state.rows.length > MAX_ROWS) {
    state.rows.pop();
    list.lastElementChild?.remove();
  }
  $('#empty').hidden = true;
  updateCount();
}

function updateCount(tookMs) {
  if (!$('#error').hidden) { $('#count').textContent = ''; return; }
  const took = tookMs != null ? ` · ${tookMs} ms` : '';
  $('#count').textContent = t('count', state.rows.length).replace(/^\d+/, fmtNum(state.rows.length)) + took;
}

function detailsHTML(r) {
  const order = ['received', 'msg_time', '_time', 'host', 'host_name', 'host_ip', 'source_type', 'audit_type', 'audit_serial', 'auid', 'user', 'exe', 'res', 'key', 'container', 'image', 'compose_project', 'compose_service', 'stream', 'unit', 'log_file', 'subsystem', 'device', 'app', 'procid', 'severity', 'facility', 'source', 'proto', 'sd', '_msg'];
  const hidden = new Set(['_stream_id', '_stream', 'sevnum', 'audit_raw']); // audit_raw: shown under the message
  if (r.msg_time) hidden.add('_time'); // same as the reception time
  const keys = order.filter((k) => r[k] != null && r[k] !== '' && !hidden.has(k))
    .concat(Object.keys(r).filter((k) => !order.includes(k) && !hidden.has(k)).sort());
  const label = {
    container: t('fContainer'), container_id: t('fContainerId'), image: t('fImage'), compose_project: t('fProject'),
    compose_service: t('fService'), stream: t('fStream'), source_type: t('fSourceType'), unit: t('fUnit'), log_file: t('fLogFile'),
    msg_time: t('fTime'), host_name: t('fHostName'), host_ip: t('fHostIp'), received: t('fReceived'), _time: t('fTime'), _msg: t('fMsg'), procid: t('fPid'), sd: t('fSd') };
  const val = (k) => (k === '_time' || k === 'received' || k === 'msg_time'
    ? `${esc(fmtFull(r[k]))} <span class="muted">(${esc(r[k])})</span>`
    : esc(r[k]));
  return '<div class="details"><dl>'
    + keys.map((k) => `<dt>${esc(label[k] || k)}</dt><dd>${val(k)}</dd>`).join('')
    + '</dl><div class="acts">'
    + (r.host ? `<button class="btn" data-act="host">${esc(t('filterHost'))}</button>` : '')
    + (r.app ? `<button class="btn" data-act="app">${esc(t('filterApp'))}</button>` : '')
    + `<button class="btn" data-act="copy">${esc(t('copyMsg'))}</button>`
    + `<button class="btn" data-act="json">${esc(t('copyJson'))}</button>`
    + '</div></div>';
}

async function copy(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Fallback for pages served over plain HTTP (not localhost).
    const ta = Object.assign(document.createElement('textarea'), { value: text });
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
  toast(t('copied'));
}

function setSelect(id, value) {
  const sel = $('#' + id);
  if (![...sel.options].some((o) => o.value === value)) sel.add(new Option(value, value));
  sel.value = value;
}

list.addEventListener('click', (ev) => {
  const row = ev.target.closest('.row');
  if (!row) return;
  const r = recOf.get(row);
  const act = ev.target.closest('[data-act]');
  if (act) {
    const a = act.dataset.act;
    if (a === 'host' || a === 'app') { setSelect(a, r[a]); onFilterChange(); }
    if (a === 'copy') copy(r._msg || '');
    if (a === 'json') copy(JSON.stringify(r, null, 2));
    return;
  }
  if (ev.target.closest('.details')) return;
  if (String(window.getSelection())) return; // selecting text: don't collapse
  const open = row.classList.toggle('open');
  const det = row.querySelector('.details');
  if (open && !det) row.insertAdjacentHTML('beforeend', detailsHTML(r));
  else if (det) det.hidden = !open;
});
list.addEventListener('keydown', (ev) => {
  if (ev.key === 'Enter' && ev.target.classList.contains('row')) ev.target.click();
});

/* ================= Column widths ================= */

// Resizable columns of the log list: width in px per column, remembered per
// browser (logstream.cols.<name>); no stored width = automatic default (CSS).
const COLS = { rcv: [60, 420], mt: [60, 420], sev: [36, 200], host: [50, 600], app: [40, 600] }; // [min, max]
const clampCol = (k, w) => Math.min(COLS[k][1], Math.max(COLS[k][0], Math.round(w)));

function applyColWidths() {
  const table = $('#table');
  for (const k of Object.keys(COLS)) {
    const w = parseInt(store.get('cols.' + k, ''), 10);
    if (w > 0) table.style.setProperty('--col-' + k, clampCol(k, w) + 'px');
    else table.style.removeProperty('--col-' + k);
  }
}

$('#listHead').addEventListener('pointerdown', (ev) => {
  const grip = ev.target.closest('.grip');
  if (!grip || ev.button !== 0) return;
  ev.preventDefault();
  const k = grip.parentElement.dataset.col;
  const x0 = ev.clientX;
  const w0 = grip.parentElement.getBoundingClientRect().width;
  const table = $('#table');
  grip.setPointerCapture(ev.pointerId);
  grip.classList.add('active');
  document.body.classList.add('col-resizing');
  const move = (e) => table.style.setProperty('--col-' + k, clampCol(k, w0 + e.clientX - x0) + 'px');
  const end = () => {
    grip.removeEventListener('pointermove', move);
    grip.removeEventListener('pointerup', end);
    grip.removeEventListener('pointercancel', end);
    grip.classList.remove('active');
    document.body.classList.remove('col-resizing');
    const w = parseInt(table.style.getPropertyValue('--col-' + k), 10);
    if (w > 0 && w !== Math.round(w0)) store.set('cols.' + k, String(w));
  };
  grip.addEventListener('pointermove', move);
  grip.addEventListener('pointerup', end);
  grip.addEventListener('pointercancel', end);
});
// Double-click on a handle: back to the automatic width.
$('#listHead').addEventListener('dblclick', (ev) => {
  const grip = ev.target.closest('.grip');
  if (!grip) return;
  store.set('cols.' + grip.parentElement.dataset.col, '');
  applyColWidths();
});
$('#resetCols').addEventListener('click', () => {
  for (const k of Object.keys(COLS)) store.set('cols.' + k, '');
  applyColWidths();
  toast(t('colsReset'));
});

// The header sticks right under the top bar, whose height changes with the layout.
new ResizeObserver(([e]) => {
  document.documentElement.style.setProperty('--topbar-h', Math.ceil(e.target.getBoundingClientRect().height) + 'px');
}).observe($('.topbar'));
applyColWidths();

/* ================= Timeline (histogram) ================= */

// Settings > Interface > Timeline, remembered per browser; the first value is the default.
const HISTO_OPTS = {
  scale: ['sqrt', 'linear', 'log'],
  height: ['m', 's', 'l'],
  color: ['severity', 'intensity', 'none'],
  render: ['bars', 'area'],
  step: ['auto', '1s', '10s', '1m', '5m', '1h', '1d'],
  refresh: ['auto', 'off', '5', '15', '30', '60'],
};
const HISTO_HEIGHTS = { s: 40, m: 80, l: 120 };
const histoPrefs = {};
for (const [k, vals] of Object.entries(HISTO_OPTS)) {
  const v = store.get('histo.' + k, vals[0]);
  histoPrefs[k] = vals.includes(v) ? v : vals[0];
}

const HISTO_LAG = 1500; // logs reach VictoriaLogs in batches (FLUSH_MS): resync a bit after a boundary
const SEV_ORDER = ['emerg', 'alert', 'crit', 'err', 'warning', 'notice', 'info', 'debug'];
// Stacked layers, bottom to top: error and above, warning, everything else.
const sevLayer = (sev) => { const n = SEV_ORDER.indexOf(sev); return n < 0 ? 2 : n <= 3 ? 0 : n === 4 ? 1 : 2; };
const SEV_LAYERS = ['err', 'warn', 'other'];
const INTENSITY = ['calm', 'burst', 'anom']; // <= 3x, > 3x, > 10x the median of the window

const histo = {
  data: null,      // see buildHisto
  reqId: 0,
  slideTimer: 0,   // end of the last interval
  refreshTimer: 0, // periodic refresh (5 s … 1 min)
  resyncTimer: 0,  // reload shortly after a new interval
  drag: null,      // {a, b}: interval indexes of a drag selection
  ticksKey: '',
  ticks: [],
};

const sliding = () => !state.zoom; // the window follows the current time

// Server answer -> dense arrays. offset = server clock - browser clock: the
// timeline runs on the server clock, like the reception times it counts.
function buildHisto(res, localNow) {
  const d = {
    step: res.step, start: res.start, count: res.count, capped: res.capped, offset: res.now - localNow,
    n: [], layers: [], sev: [],
  };
  for (let i = 0; i < d.count; i++) { d.n.push(0); d.layers.push([0, 0, 0]); d.sev.push({}); }
  for (const b of res.buckets || []) {
    if (!(b.i >= 0 && b.i < d.count)) continue;
    d.n[b.i] = b.n;
    for (const [s, c] of Object.entries(b.sev || {})) {
      d.sev[b.i][s] = c;
      d.layers[b.i][sevLayer(s)] += c;
    }
  }
  return d;
}

async function refreshHisto() {
  const id = ++histo.reqId;
  clearTimeout(histo.resyncTimer);
  const p = params();
  p.set('step', histoPrefs.step);
  p.set('tz', timePrefs.tz || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC');
  const t0 = Date.now();
  let res = null;
  try { res = await api('/api/histogram?' + p); } catch { /* shown as an empty timeline */ }
  if (id !== histo.reqId) return;
  histo.data = res && buildHisto(res, (t0 + Date.now()) / 2);
  renderHisto();
  scheduleSlide();
  scheduleRefresh();
}

// Moves the window k intervals forward (new empty intervals on the right).
function histoSlide(k, resync) {
  const d = histo.data;
  k = Math.min(Math.max(k, 0), d.count);
  if (!k) return;
  d.start += k * d.step;
  for (const a of [d.n, d.layers, d.sev]) a.splice(0, k);
  for (let j = 0; j < k; j++) { d.n.push(0); d.layers.push([0, 0, 0]); d.sev.push({}); }
  if (resync && !document.hidden) {
    clearTimeout(histo.resyncTimer);
    histo.resyncTimer = setTimeout(refreshHisto, HISTO_LAG);
  }
}

const liveOn = () => state.live && liveAvailable();

// At the end of the last interval: slide, so that the axis keeps up with the
// clock even without new logs. Off only when nothing refreshes the timeline.
function scheduleSlide() {
  clearTimeout(histo.slideTimer);
  const d = histo.data;
  if (!d || !sliding() || document.hidden || (!liveOn() && histoPrefs.refresh === 'off')) return;
  const delay = d.start + d.count * d.step - (Date.now() + d.offset);
  histo.slideTimer = setTimeout(() => {
    const x = histo.data;
    if (!x) return;
    const late = Date.now() + x.offset - (x.start + x.count * x.step);
    histoSlide(Math.floor(late / x.step) + 1, liveOn() || histoPrefs.refresh === 'auto');
    renderHisto();
    scheduleSlide();
  }, Math.min(Math.max(delay, 50), 2 ** 31 - 1));
}

function scheduleRefresh() {
  clearTimeout(histo.refreshTimer);
  const sec = Number(histoPrefs.refresh);
  if (!sec || !histo.data || !sliding() || document.hidden) return;
  histo.refreshTimer = setTimeout(refreshHisto, sec * 1000);
}

function scheduleHisto() {
  scheduleSlide();
  scheduleRefresh();
}

// Live view: count the new messages right away; the next resync corrects the
// approximations (messages skipped by the stream, late arrivals).
function histoAddLive(rows, skipped) {
  const d = histo.data;
  if (!d || !sliding() || document.hidden) return;
  let changed = false;
  for (const r of rows) {
    const t = parseTime(r.received || r._time).getTime();
    if (isNaN(t)) continue;
    let i = Math.floor((t - d.start) / d.step);
    if (i >= d.count) {
      histoSlide(i - d.count + 1, true);
      i = Math.floor((t - d.start) / d.step);
    }
    if (i < 0 || i >= d.count) continue;
    const sev = r.severity || 'info';
    d.n[i]++;
    d.layers[i][sevLayer(sev)]++;
    d.sev[i][sev] = (d.sev[i][sev] || 0) + 1;
    changed = true;
  }
  if (skipped > 0) {
    d.n[d.count - 1] += skipped;
    d.layers[d.count - 1][2] += skipped;
    changed = true;
  }
  if (changed) renderHisto();
}

/* ---- Rendering ---- */

function histoScale(n, max) {
  if (n <= 0 || max <= 0) return 0;
  const v = histoPrefs.scale === 'linear' ? n / max
    : histoPrefs.scale === 'log' ? Math.log1p(n) / Math.log1p(max)
      : Math.sqrt(n / max);
  return Math.max(v, 0.04); // a single log stays visible
}

// Median of the non-empty intervals (0 when too few to judge).
function histoMedian(d) {
  const v = d.n.filter((x) => x > 0).sort((a, b) => a - b);
  if (v.length < 3) return 0;
  const m = v.length >> 1;
  return v.length % 2 ? v[m] : (v[m - 1] + v[m]) / 2;
}
const intensityOf = (n, med) => (!med || !n ? 0 : n > 10 * med ? 2 : n > 3 * med ? 1 : 0);

// Local wall-clock offset (ms) of a date in the selected time zone.
function zoneOffset(ms) {
  const m = /^([+-])(\d\d):(\d\d)$/.exec(zoneParts(new Date(ms)).off);
  return m ? (m[1] === '-' ? -1 : 1) * (Number(m[2]) * 60 + Number(m[3])) * 60000 : 0;
}

const TICK_STEPS = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400, 172800, 604800]
  .map((s) => s * 1000);
const dayLabel = (p) => (lang === 'fr' ? `${p.D}/${p.M}` : `${p.M}/${p.D}`);

// Vertical graduations on interval boundaries, at least ~70 px apart, on
// round local times; labels hh:mm:ss, hh:mm or dd/mm depending on the spacing.
function histoTicks(d, width) {
  const key = [d.start, d.step, d.count, width, timePrefs.tz, lang].join();
  if (histo.ticksKey === key) return histo.ticks;
  const pxPerMs = width / (d.count * d.step);
  const every = TICK_STEPS.find((s) => s >= d.step && s % d.step === 0 && s * pxPerMs >= 70)
    || d.step * Math.max(1, Math.ceil(70 / (pxPerMs * d.step)));
  const endMs = d.start + d.count * d.step;
  const off0 = zoneOffset(d.start);
  const sameOffset = off0 === zoneOffset(endMs); // no DST change in the window
  const ticks = [];
  for (let i = 0; i <= d.count; i++) {
    const t = d.start + i * d.step;
    const wall = t + (sameOffset ? off0 : zoneOffset(t));
    if (((wall % every) + every) % every !== 0) continue;
    const x = i / d.count;
    if (x < 0.03 || x > 0.97) continue; // labels would be cut at the edges
    const p = zoneParts(new Date(t));
    const midnight = wall % 864e5 === 0;
    const label = every >= 864e5 || (midnight && every >= 6e4) ? dayLabel(p)
      : every < 6e4 ? `${p.h}:${p.m}:${p.s}` : `${p.h}:${p.m}`;
    ticks.push({ x, label, day: midnight });
  }
  histo.ticksKey = key;
  histo.ticks = ticks;
  return ticks;
}

// "01/10 19:52 → 20:07" (with seconds for short intervals).
function fmtSpan(from, to, withSec) {
  const a = zoneParts(new Date(from));
  const b = zoneParts(new Date(to - 1));
  const e = zoneParts(new Date(to));
  const hm = (p) => `${p.h}:${p.m}${withSec ? ':' + p.s : ''}`;
  return `${dayLabel(a)} ${hm(a)} → ${a.D === b.D && a.M === b.M ? '' : dayLabel(e) + ' '}${hm(e)}`;
}

function fmtStep(ms) {
  const units = [[864e5, t('unitDay')], [36e5, 'h'], [6e4, 'min'], [1e3, 's']];
  const [size, unit] = units.find(([u]) => ms % u === 0) || units[3];
  return `${fmtNum(ms / size)} ${unit}`;
}

function histoSVG(d) {
  const N = d.count;
  const max = Math.max(0, ...d.n);
  if (!max) return '';
  const area = histoPrefs.render === 'area';
  const mode = histoPrefs.color;
  const med = mode === 'intensity' ? histoMedian(d) : 0;
  const top = (n) => histoScale(n, max) * 100; // height in % of the plot
  const y = (h) => (100 - h).toFixed(2);

  if (!area) {
    const g = N > 120 ? 0.08 : 0.15;
    const groups = {};
    const rect = (cls, i, y0, h) => {
      if (h <= 0) return;
      groups[cls] = (groups[cls] || '') + `<rect x="${(i + g).toFixed(2)}" y="${y(y0 + h)}" width="${(1 - 2 * g).toFixed(2)}" height="${h.toFixed(2)}"/>`;
    };
    for (let i = 0; i < N; i++) {
      const n = d.n[i];
      if (!n) continue;
      const h = top(n);
      if (mode === 'severity') {
        let y0 = 0;
        d.layers[i].forEach((c, k) => { const hk = (h * c) / n; rect('l-' + SEV_LAYERS[k], i, y0, hk); y0 += hk; });
      } else {
        rect(mode === 'intensity' ? 'l-' + INTENSITY[intensityOf(n, med)] : 'l-all', i, 0, h);
      }
    }
    return Object.entries(groups).map(([cls, r]) => `<g class="${cls}">${r}</g>`).join('');
  }

  // Area: one point per interval center, stacked by severity when asked.
  const line = (vals) => vals.map((v, i) => `${i === 0 ? '0' : (i + 0.5).toFixed(2)},${y(v)}`).join(' L')
    + ` L${N},${y(vals[N - 1])}`;
  const back = (vals) => [...vals.keys()].reverse().map((i) => `L${i === 0 ? '0' : (i + 0.5).toFixed(2)},${y(vals[i])}`).join(' ');
  if (mode === 'severity') {
    let below = d.n.map(() => 0);
    let out = '';
    for (let k = 0; k < 3; k++) {
      const cum = d.n.map((n, i) => (n ? below[i] + (top(n) * d.layers[i][k]) / n : 0));
      out += `<path class="l-${SEV_LAYERS[k]}" d="M${line(cum)} L${N},${y(below[N - 1])} ${back(below)} Z"/>`;
      below = cum;
    }
    return out;
  }
  const tops = d.n.map(top);
  const path = `M0,100 L${line(tops)} L${N},100 Z`;
  if (mode !== 'intensity') return `<path class="l-all" d="${path}"/>`;
  // Intensity: a horizontal gradient with one color band per interval.
  let stops = '';
  for (let i = 0; i < N; i++) {
    const c = `var(--h-${INTENSITY[intensityOf(d.n[i], med)]})`;
    stops += `<stop offset="${i / N}" style="stop-color:${c}"/><stop offset="${(i + 1) / N}" style="stop-color:${c}"/>`;
  }
  return `<defs><linearGradient id="hGrad" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="${N}" y2="0">${stops}</linearGradient></defs>`
    + `<path d="${path}" fill="url(#hGrad)"/>`;
}

function histoLegend() {
  const items = histoPrefs.color === 'severity' ? SEV_LAYERS.map((k) => [k, t('hLeg_' + k)])
    : histoPrefs.color === 'intensity' ? INTENSITY.map((k) => [k, t('hLeg_' + k)]) : [];
  return items.map(([k, label]) => `<span><i class="sw" style="background:var(--h-${k})"></i>${esc(label)}</span>`).join('');
}

function renderHisto() {
  const el = $('#histo');
  const d = histo.data;
  el.style.setProperty('--histo-h', HISTO_HEIGHTS[histoPrefs.height] + 'px');
  el.hidden = !d;
  if (!d) return;
  const plot = el.querySelector('.h-plot');
  const svg = el.querySelector('.h-svg');
  svg.setAttribute('viewBox', `0 0 ${d.count} 100`);
  svg.innerHTML = histoSVG(d);
  const ticks = histoTicks(d, plot.clientWidth || 1000);
  el.querySelector('.h-grid').innerHTML = ticks.map((k) => `<i class="${k.day ? 'day' : ''}" style="left:${(k.x * 100).toFixed(3)}%"></i>`).join('');
  el.querySelector('.h-axis').innerHTML = ticks.map((k) => `<span class="${k.day ? 'day' : ''}" style="left:${(k.x * 100).toFixed(3)}%">${esc(k.label)}</span>`).join('');
  const max = Math.max(0, ...d.n);
  el.querySelector('.h-meta').innerHTML = `<span class="h-leg">${histoLegend()}</span><span class="sp"></span>`
    + `<span>${esc(t('histoMax', fmtNum(max)))} · ${esc(t('hDivision', fmtStep(d.step)))}`
    + (d.capped ? ` <span class="warn" title="${esc(t('hCappedTitle'))}">${esc(t('hCapped'))}</span>` : '') + '</span>'
    + (state.zoom ? `<button type="button" class="link-btn" data-act="unzoom">${esc(t('hUnzoom'))}</button>` : '');
  if (histo.tipIdx != null) showHistoTip(histo.tipIdx);
}

/* ---- Tooltip, zoom ---- */

function histoIdx(ev) {
  const d = histo.data;
  const r = $('#histo .h-plot').getBoundingClientRect();
  return Math.min(d.count - 1, Math.max(0, Math.floor(((ev.clientX - r.left) / r.width) * d.count)));
}

function showHistoTip(i, clientX) {
  const d = histo.data;
  const tip = $('#histo .h-tip');
  const hover = $('#histo .h-hover');
  if (!d || i >= d.count) { hideHistoTip(); return; }
  histo.tipIdx = i;
  const from = d.start + i * d.step;
  const n = d.n[i];
  let html = `<div class="hd">${esc(fmtSpan(from, from + d.step, d.step < 6e4))}</div>`
    + `<div class="ln"><span>${esc(t('hTotal'))}</span><b>${fmtNum(n)}</b></div>`;
  if (histoPrefs.color === 'intensity' && n) {
    const med = histoMedian(d);
    if (med) html += `<div class="ln muted"><span>${esc(t('hLeg_' + INTENSITY[intensityOf(n, med)]))}</span><span>${esc(t('hRatio', (n / med).toLocaleString(t('locale'), { maximumFractionDigits: 1 })))}</span></div>`;
  }
  const sevs = Object.keys(d.sev[i]).sort((a, b) => SEV_ORDER.indexOf(a) - SEV_ORDER.indexOf(b));
  for (const s of sevs) {
    html += `<div class="ln"><span><i class="sw" style="background:var(--sev-${SEV_CLASS[s] || 'info'})"></i>${esc(s)}</span><span>${fmtNum(d.sev[i][s])}</span></div>`;
  }
  const other = n - sevs.reduce((sum, s) => sum + d.sev[i][s], 0);
  if (other > 0) html += `<div class="ln muted"><span>${esc(t('hUnshown'))}</span><span>${fmtNum(other)}</span></div>`;
  tip.innerHTML = html;
  tip.hidden = false;
  const plot = $('#histo .h-plot');
  const w = plot.clientWidth;
  hover.hidden = false;
  hover.style.left = (i / d.count) * 100 + '%';
  hover.style.width = 100 / d.count + '%';
  if (clientX != null) histo.tipX = clientX - plot.getBoundingClientRect().left;
  const x = histo.tipX ?? ((i + 0.5) / d.count) * w;
  tip.style.left = Math.min(Math.max(x + 12 + plot.offsetLeft, 0), plot.offsetLeft + w - tip.offsetWidth) + 'px';
  tip.style.top = plot.offsetTop + plot.offsetHeight + 4 + 'px';
}

function hideHistoTip() {
  histo.tipIdx = null;
  $('#histo .h-tip').hidden = true;
  $('#histo .h-hover').hidden = true;
}

function showHistoSel() {
  const d = histo.data;
  const sel = $('#histo .h-sel');
  const a = Math.min(histo.drag.a, histo.drag.b);
  const b = Math.max(histo.drag.a, histo.drag.b);
  sel.hidden = false;
  sel.style.left = (a / d.count) * 100 + '%';
  sel.style.width = ((b - a + 1) / d.count) * 100 + '%';
}

/* ---- Display mode: Stream or Time range ---- */

// datetime-local fields hold a wall time in the chosen time zone.
function toWall(ms, sec) {
  const p = zoneParts(new Date(ms));
  return `${p.Y}-${p.M}-${p.D}T${p.h}:${p.m}${sec ? ':' + p.s : ''}`;
}

function zoneOffsetMs(ms) {
  const m = /^([+-])(\d\d):(\d\d)$/.exec(zoneParts(new Date(ms)).off);
  return m ? (m[1] === '-' ? -1 : 1) * (m[2] * 36e5 + m[3] * 6e4) : 0;
}

function fromWall(v) {
  const m = /^(\d{4})-(\d\d)-(\d\d)T(\d\d):(\d\d)(?::(\d\d))?/.exec(v || '');
  if (!m) return NaN;
  const wall = Date.UTC(m[1], m[2] - 1, m[3], m[4], m[5], m[6] || 0);
  const guess = wall - zoneOffsetMs(wall);
  return wall - zoneOffsetMs(guess); // second pass: right offset around DST changes
}

const serverNow = () => Date.now() + (histo.data ? histo.data.offset : 0);

function renderPeriod() {
  const on = !!state.zoom;
  for (const b of $('#viewMode').querySelectorAll('[data-view]')) {
    b.setAttribute('aria-checked', String((b.dataset.view === 'period') === on));
  }
  $('#range').hidden = on;
  $('#period').hidden = !on;
  if (!on) return;
  const { from, to } = state.zoom;
  const sec = from % 6e4 !== 0 || to % 6e4 !== 0;
  for (const [el, v] of [[$('#perFrom'), from], [$('#perTo'), to]]) {
    el.step = sec ? 1 : 60;
    el.classList.remove('bad');
    if (document.activeElement !== el) el.value = toWall(v, sec);
  }
  $('#perNext').disabled = to >= serverNow();
}

function zoomTo(from, to) {
  state.zoom = { from: Math.round(from), to: Math.round(to) };
  store.set('period', JSON.stringify(state.zoom));
  renderPeriod();
  hideHistoTip();
  refresh();
}

function unzoom() {
  state.zoom = null;
  store.set('period', '');
  renderPeriod();
  $('#range').value = store.get('range', '1h');
  if (!$('#range').value) $('#range').value = '1h';
  refresh();
}

// Entering Time range mode starts from what the stream was showing.
function enterPeriod() {
  if (state.zoom) return;
  const d = histo.data;
  if (d) return zoomTo(d.start, d.start + d.count * d.step);
  const to = Math.ceil(serverNow() / 6e4) * 6e4;
  zoomTo(to - (RANGE_MS[$('#range').value] || 36e5), to);
}

$('#viewMode').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-view]');
  if (!b) return;
  if (b.dataset.view === 'period') enterPeriod();
  else if (state.zoom) unzoom();
});

const onPeriodInput = debounce(() => {
  if (!state.zoom) return;
  const from = fromWall($('#perFrom').value);
  const to = fromWall($('#perTo').value);
  const ok = !isNaN(from) && !isNaN(to) && to > from;
  $('#perFrom').classList.toggle('bad', isNaN(from) || (!isNaN(to) && !ok));
  $('#perTo').classList.toggle('bad', isNaN(to) || (!isNaN(from) && !ok));
  if (ok && (from !== state.zoom.from || to !== state.zoom.to)) zoomTo(from, to);
}, 500);
for (const id of ['perFrom', 'perTo']) {
  $('#' + id).addEventListener('input', onPeriodInput);
  $('#' + id).addEventListener('keydown', (ev) => { if (ev.key === 'Enter') ev.target.blur(); });
  $('#' + id).addEventListener('blur', () => { if (!$('#' + id).classList.contains('bad')) renderPeriod(); });
}

// ◀ ▶ move by the width of the range, − doubles it around its middle.
function shiftPeriod(dir) {
  const { from, to } = state.zoom;
  const w = to - from;
  zoomTo(from + dir * w, to + dir * w);
}
$('#perPrev').addEventListener('click', () => shiftPeriod(-1));
$('#perNext').addEventListener('click', () => shiftPeriod(1));
$('#perOut').addEventListener('click', () => {
  const { from, to } = state.zoom;
  const w = to - from;
  let a = from - w / 2;
  let b = to + w / 2;
  const now = serverNow();
  const end = Math.max(to, now); // widen into the past rather than the future
  if (b > end) { a -= b - end; b = end; }
  const r = w >= 12e4 ? 6e4 : 1e3; // whole minutes, or seconds for short ranges
  zoomTo(Math.round(a / r) * r, Math.round(b / r) * r);
});

{
  const el = $('#histo');
  el.innerHTML = '<div class="h-plot"><svg class="h-svg" preserveAspectRatio="none" aria-hidden="true"></svg>'
    + '<div class="h-grid"></div><div class="h-hover" hidden></div><div class="h-sel" hidden></div></div>'
    + '<div class="h-axis"></div><div class="h-meta"></div><div class="h-tip" role="tooltip" hidden></div>';
  el.hidden = true;
  const plot = el.querySelector('.h-plot');
  plot.addEventListener('pointerdown', (ev) => {
    if (ev.button !== 0 || !histo.data) return;
    const i = histoIdx(ev);
    histo.drag = { a: i, b: i };
    plot.setPointerCapture(ev.pointerId);
    showHistoSel();
  });
  plot.addEventListener('pointermove', (ev) => {
    if (!histo.data) return;
    const i = histoIdx(ev);
    if (histo.drag) { histo.drag.b = i; showHistoSel(); }
    showHistoTip(i, ev.clientX);
  });
  plot.addEventListener('pointerup', () => {
    const d = histo.data;
    const dr = histo.drag;
    histo.drag = null;
    el.querySelector('.h-sel').hidden = true;
    if (!dr || !d) return;
    const a = Math.min(dr.a, dr.b);
    const b = Math.max(dr.a, dr.b);
    zoomTo(d.start + a * d.step, d.start + (b + 1) * d.step); // click = one interval, drag = the selection
  });
  plot.addEventListener('pointercancel', () => { histo.drag = null; el.querySelector('.h-sel').hidden = true; });
  plot.addEventListener('pointerleave', () => { if (!histo.drag) hideHistoTip(); });
  el.querySelector('.h-meta').addEventListener('click', (ev) => {
    if (ev.target.closest('[data-act="unzoom"]')) unzoom();
  });
  plot.title = '';
  window.addEventListener('resize', debounce(() => renderHisto(), 200));
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      clearTimeout(histo.slideTimer);
      clearTimeout(histo.refreshTimer);
      clearTimeout(histo.resyncTimer);
    } else if (histo.data && sliding()) {
      refreshHisto(); // catch up on what happened while the tab was hidden
    }
  });
}

/* ---- Settings > Interface > Timeline ---- */

const HISTO_SELECT_LABELS = {
  step: (v) => (v === 'auto' ? t('hAuto') : fmtStep({ s: 1e3, m: 6e4, h: 36e5, d: 864e5 }[v.slice(-1)] * parseInt(v, 10))),
  refresh: (v) => (v === 'auto' ? t('hRefreshAuto') : v === 'off' ? t('hOff') : fmtStep(Number(v) * 1000)),
};

function renderHistoSettings() {
  for (const seg of document.querySelectorAll('.seg[data-histo]')) {
    for (const b of seg.querySelectorAll('[data-v]')) b.setAttribute('aria-checked', String(b.dataset.v === histoPrefs[seg.dataset.histo]));
  }
  for (const sel of document.querySelectorAll('select[data-histo]')) {
    const k = sel.dataset.histo;
    sel.innerHTML = HISTO_OPTS[k].map((v) => `<option value="${v}">${esc(HISTO_SELECT_LABELS[k](v))}</option>`).join('');
    sel.value = histoPrefs[k];
  }
}

function setHistoPref(k, v) {
  if (!HISTO_OPTS[k] || !HISTO_OPTS[k].includes(v)) return;
  histoPrefs[k] = v;
  store.set('histo.' + k, v);
  renderHistoSettings();
  if (k === 'step') refreshHisto();
  else if (k === 'refresh') { scheduleHisto(); if (v !== 'off') refreshHisto(); }
  else renderHisto();
}

document.addEventListener('click', (ev) => {
  const b = ev.target.closest('.seg[data-histo] [data-v]');
  if (b) setHistoPref(b.closest('.seg').dataset.histo, b.dataset.v);
});
for (const sel of document.querySelectorAll('select[data-histo]')) {
  sel.addEventListener('change', () => setHistoPref(sel.dataset.histo, sel.value));
}

/* ================= Search ================= */

function setError(msg) {
  const el = $('#error');
  el.textContent = msg || '';
  el.hidden = !msg;
}

async function refresh() {
  const id = ++state.reqId;
  syncControls();
  compileMatchers();
  document.body.classList.add('busy');
  connectLive();
  try {
    const res = await api(`/api/logs?${params()}&limit=${PAGE_SIZE}`);
    if (id !== state.reqId) return;
    setError(null);
    state.rows = res.rows || [];
    state.pending = [];
    state.tookMs = res.took_ms;
    $('#newPill').hidden = true;
    renderList();
    updateCount(state.tookMs);
  } catch (e) {
    if (id !== state.reqId) return;
    setError(t('searchError') + e.message);
    state.rows = [];
    renderList();
    updateCount();
  } finally {
    if (id === state.reqId) document.body.classList.remove('busy');
  }
  refreshHisto();
}

const onFilterChange = (ev) => {
  if (!state.zoom) store.set('range', $('#range').value);
  store.set('severity', $('#severity').value);
  refresh();
};

$('#q').addEventListener('input', debounce(refresh, 350));
$('#q').addEventListener('keydown', (ev) => {
  if (ev.key === 'Enter') refresh();
  if (ev.key === 'Escape') { ev.target.value = ''; refresh(); }
});
for (const id of ['range', 'severity', 'host', 'app', 'source']) $('#' + id).addEventListener('change', onFilterChange);

$('#mode').addEventListener('click', () => {
  state.mode = state.mode === 'simple' ? 'logsql' : 'simple';
  store.set('mode', state.mode);
  refresh();
  $('#q').focus();
});

document.addEventListener('keydown', (ev) => {
  if (ev.key === '/' && !/INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName) && !$('#settingsDlg').open) {
    ev.preventDefault();
    $('#q').focus();
  }
});

/* ================= Live view (SSE) ================= */

function setConn(cls, key) {
  state.conn = [cls, key];
  const el = $('#conn');
  el.className = 'conn ' + cls;
  el.textContent = t(key);
}

function connectLive() {
  if (state.es) { state.es.close(); state.es = null; }
  if (!state.live || !liveAvailable()) {
    setConn('', !state.live ? 'connPaused' : state.zoom ? 'connZoom' : 'connLogsql');
    return;
  }
  const es = new EventSource('/api/stream?' + params());
  state.es = es;
  setConn('', 'connConnecting');
  es.onopen = () => setConn('ok', 'connLive');
  es.onerror = () => setConn('ko', 'connRetry');
  es.onmessage = (ev) => {
    const data = JSON.parse(ev.data);
    const rows = (data.rows || []).reverse(); // newest first
    if (data.skipped) toast(t('skipped', fmtNum(data.skipped)));
    histoAddLive(rows, data.skipped);
    if (window.scrollY > 80) {
      state.pending = rows.concat(state.pending).slice(0, MAX_ROWS);
      const pill = $('#newPill');
      pill.textContent = t('newLogs', state.pending.length).replace(/\d+/, fmtNum(state.pending.length));
      pill.hidden = false;
    } else {
      prependRows(rows);
    }
  };
}

function flushPending() {
  if (!state.pending.length) return;
  prependRows(state.pending);
  state.pending = [];
  $('#newPill').hidden = true;
}

$('#newPill').addEventListener('click', () => {
  flushPending();
  window.scrollTo({ top: 0, behavior: 'smooth' });
});
window.addEventListener('scroll', debounce(() => { if (window.scrollY <= 80) flushPending(); }, 150), { passive: true });

// "Back to top" button, shown once the page has scrolled a little.
const syncToTop = () => { $('#toTop').hidden = window.scrollY < 400; };
window.addEventListener('scroll', syncToTop, { passive: true });
$('#toTop').addEventListener('click', () => {
  const smooth = !matchMedia('(prefers-reduced-motion: reduce)').matches;
  window.scrollTo({ top: 0, behavior: smooth ? 'smooth' : 'auto' });
});
syncToTop();

$('#live').addEventListener('click', () => {
  state.live = !state.live;
  store.set('live', state.live ? '1' : '0');
  if (state.live) refresh(); // catch up on what arrived while paused
  else { syncControls(); connectLive(); scheduleHisto(); }
});

/* ================= Host / app lists, stats ================= */

async function loadFacets() {
  try {
    const f = await api('/api/facets');
    for (const id of ['host', 'app']) {
      const sel = $('#' + id);
      const cur = sel.value;
      const first = sel.options[0];
      sel.replaceChildren(first, ...(f[id] || []).map((x) => new Option(x.label || x.value, x.value)));
      if (cur) setSelect(id, cur);
    }
    addFacetValues(state.rows);
  } catch { /* VictoriaLogs not ready yet: retried later */ }
}

function renderStats() {
  const el = $('#stats');
  const s = state.stats;
  if (!s) { el.textContent = t('unreachable'); return; }
  const n = { received: fmtNum(s.received), ingested: fmtNum(s.ingested), dropped: '%DROPPED%', queue: fmtNum(s.queue) };
  const dropped = s.dropped ? `<span class="bad">${fmtNum(s.dropped)}</span>` : fmtNum(0);
  let html = esc(t('stats', n)).replace('%DROPPED%', dropped);
  if (s.spooled) html += ` · <span class="warn">${esc(t('spooled', fmtNum(s.spooled)))}</span>`;
  if (s.lastError) html += ` · <span class="bad">${esc(t('storageErr') + s.lastError)}</span>`;
  el.innerHTML = html;
}

async function loadStats() {
  try { state.stats = await api('/api/stats'); } catch { state.stats = null; }
  if (state.stats && state.stats.exportMax) exportMax = state.stats.exportMax;
  renderStats();
}

/* ================= Date & time settings ================= */

function renderTimeSettings() {
  const now = new Date();
  const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'local';
  const offsetOf = (z) => 'UTC' + zoneParts(now, z).off;
  const groups = t('tzGroups');

  let html = `<option value="">${esc(t('tzBrowser', browserZone))}</option>`;
  for (const [group, zones] of Object.entries(TIME_ZONES)) {
    html += `<optgroup label="${esc(groups[group] || group)}">`;
    for (const z of zones) {
      if (!validTimeZone(z)) continue;
      html += `<option value="${z}">${esc(z.replace(/_/g, ' '))} (${offsetOf(z)})</option>`;
    }
    html += '</optgroup>';
  }
  const tzSel = $('#tzSelect');
  tzSel.innerHTML = html;
  tzSel.value = timePrefs.tz;

  const fmtSel = $('#fmtSelect');
  fmtSel.innerHTML = TIME_FORMATS.map((f) => `<option value="${f.id}">${esc(f.label || t('fmtEpoch'))}</option>`).join('');
  fmtSel.value = timePrefs.fmt;
  updateTimePreview();
}

function updateTimePreview() {
  $('#timePreview').textContent = t('timePreview') + fmtWith(timePrefs.fmt, new Date());
}

function onTimePrefsChange() {
  timePrefs.tz = $('#tzSelect').value;
  timePrefs.fmt = $('#fmtSelect').value;
  store.set('tz', timePrefs.tz);
  store.set('timefmt', timePrefs.fmt);
  updateTimePreview();
  renderList();
  renderPeriod();
  refreshHisto(); // intervals are aligned on the local time
}
$('#tzSelect').addEventListener('change', onTimePrefsChange);
$('#fmtSelect').addEventListener('change', onTimePrefsChange);
setInterval(() => { if ($('#settingsDlg').open) updateTimePreview(); }, 1000);

/* ================= CSV export ================= */

let exportMax = 100000; // EXPORT_MAX on the server, refreshed from /api/stats

function setExportMenu(open) {
  $('#exportMenu').hidden = !open;
  $('#exportBtn').setAttribute('aria-expanded', String(open));
  if (open) $('#exportNote').textContent = t('exportNote', fmtNum(exportMax));
}

async function exportCSV(excel) {
  const p = params();
  p.set('tz', timePrefs.tz || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC');
  if (excel) p.set('excel', '1');
  const btn = $('#exportBtn');
  const lbl = btn.querySelector('.lbl');
  btn.disabled = true;
  lbl.textContent = t('exporting');
  try {
    const res = await fetch('/api/export.csv?' + p);
    if (!res.ok) {
      const text = await res.text();
      let data = null;
      try { data = JSON.parse(text); } catch { /* not JSON */ }
      throw apiError(res, text, data);
    }
    const blob = await res.blob();
    const name = ((res.headers.get('Content-Disposition') || '').match(/filename="([^"]+)"/) || [])[1] || 'logstream.csv';
    const url = URL.createObjectURL(blob);
    const a = Object.assign(document.createElement('a'), { href: url, download: name });
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 10000);
    toast(t('exportDone', name));
  } catch (e) {
    toast(t('exportError') + e.message);
  } finally {
    btn.disabled = false;
    lbl.textContent = t('export');
  }
}

$('#exportBtn').addEventListener('click', () => setExportMenu($('#exportMenu').hidden));
$('#exportMenu').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-export]');
  if (!b) return;
  setExportMenu(false);
  exportCSV(b.dataset.export === 'excel');
});
document.addEventListener('click', (ev) => { if (!ev.target.closest('.export')) setExportMenu(false); });
document.addEventListener('keydown', (ev) => { if (ev.key === 'Escape') setExportMenu(false); });

/* ================= Syslog source ================= */

let syslogState = null;

async function loadSyslog() {
  try { syslogState = await api('/api/syslog'); } catch { syslogState = null; }
  renderSyslog();
}

function renderSyslog() {
  const s = syslogState;
  const status = $('#syslogStatus');
  if (!s) { status.textContent = ''; return; }
  $('#syslogEnabled').checked = s.enabled;
  for (const b of document.querySelectorAll('#syslogProtos [data-proto]')) {
    const on = !!s[b.dataset.proto];
    b.classList.toggle('on', on);
    b.setAttribute('aria-pressed', String(on));
    b.disabled = !s.enabled;
  }
  $('#syslogProtos').classList.toggle('off', !s.enabled);
  $('#syslogPortVal').textContent = s.port || s.internalAddr;
  const internal = String(s.internalAddr || '').replace(/^:/, '');
  $('#syslogPortHelp').textContent = t('syslogPortHelp', internal);

  const errs = [];
  if (s.udpError) errs.push(t('syslogErr', 'UDP') + s.udpError);
  if (s.tcpError) errs.push(t('syslogErr', 'TCP') + s.tcpError);
  const protos = [s.udpListening && 'UDP', s.tcpListening && 'TCP'].filter(Boolean).join(' + ');
  if (errs.length) {
    status.textContent = errs.join(' · ');
    status.className = 'docker-status bad';
  } else if (!s.enabled) {
    status.textContent = t('syslogOff');
    status.className = 'docker-status muted';
  } else if (!protos) {
    status.textContent = t('syslogNone');
    status.className = 'docker-status muted';
  } else {
    status.textContent = t('syslogListening', { port: s.port || internal, protos });
    status.className = 'docker-status good';
  }
}

async function configureSyslog(change) {
  const s = syslogState || { enabled: true, udp: true, tcp: true };
  const body = { enabled: s.enabled, udp: s.udp, tcp: s.tcp, ...change };
  syslogState = { ...s, ...body };
  renderSyslog();
  try { syslogState = await api('/api/syslog', { method: 'PUT', body }); } catch (e) { toast(e.message); }
  renderSyslog();
}

$('#syslogEnabled').addEventListener('change', (ev) => configureSyslog({ enabled: ev.target.checked }));
$('#syslogProtos').addEventListener('click', (ev) => {
  const b = ev.target.closest('[data-proto]');
  if (!b || b.disabled || !syslogState) return;
  configureSyslog({ [b.dataset.proto]: !syslogState[b.dataset.proto] });
});

/* ================= Host system logs ================= */

let hostState = null;

async function loadHost() {
  try { hostState = await api('/api/hostlogs'); } catch { hostState = null; }
  renderHost();
}

function renderHost() {
  const s = hostState;
  const status = $('#hostStatus');
  if (!s) { status.textContent = ''; return; }
  $('#hostEnabled').checked = s.enabled;
  let text = t('hostWaiting');
  let cls = 'muted';
  if (!s.enabled) {
    text = t('hostOff');
  } else if (s.code) {
    text = t('host_' + s.code) + (s.code === 'read' || s.code === 'permission' ? s.error || '' : '');
    cls = 'bad';
  } else if (s.mode === 'journal') {
    text = t('hostJournal', { n: s.files, r: fmtNum(s.read) }) + (s.compressed ? t('hostCompressed', fmtNum(s.compressed)) : '');
    cls = 'good';
  } else if (s.mode === 'files') {
    text = t('hostFiles', { n: s.files, r: fmtNum(s.read) });
    cls = 'good';
  }
  status.textContent = text;
  status.className = 'docker-status ' + cls;
}

async function configureHost(enabled) {
  hostState = { ...(hostState || {}), enabled };
  renderHost();
  try { hostState = await api('/api/hostlogs', { method: 'PUT', body: { enabled } }); } catch (e) { toast(e.message); }
  renderHost();
  setTimeout(loadHost, 1500); // the first scan runs in the background
}

$('#hostEnabled').addEventListener('change', (ev) => configureHost(ev.target.checked));

/* ================= Kernel messages (dmesg) ================= */

let dmesgState = null;

async function loadDmesg() {
  try { dmesgState = await api('/api/dmesg'); } catch { dmesgState = null; }
  renderDmesg();
}

function renderDmesg() {
  const s = dmesgState;
  const status = $('#dmesgStatus');
  if (!s) { status.textContent = ''; return; }
  $('#dmesgEnabled').checked = s.enabled;
  let text = t('hostWaiting');
  let cls = 'muted';
  if (!s.enabled) {
    text = t('dmesgOff');
  } else if (s.code) {
    text = t('dmesg_' + s.code) + (s.code === 'read' || s.code === 'permission' ? s.error || '' : '');
    cls = 'bad';
  } else if (s.open) {
    text = t('dmesgOk', { d: s.device, r: fmtNum(s.read) }) + (s.lost ? t('dmesgLost', fmtNum(s.lost)) : '');
    cls = 'good';
  }
  status.textContent = text;
  status.className = 'docker-status ' + cls;
}

async function configureDmesg(enabled) {
  dmesgState = { ...(dmesgState || {}), enabled };
  renderDmesg();
  try { dmesgState = await api('/api/dmesg', { method: 'PUT', body: { enabled } }); } catch (e) { toast(e.message); }
  renderDmesg();
  setTimeout(loadDmesg, 1500); // the device is opened in the background
}

$('#dmesgEnabled').addEventListener('change', (ev) => configureDmesg(ev.target.checked));

/* ================= Linux audit (auditd) ================= */

let auditState = null;

async function loadAudit() {
  try { auditState = await api('/api/audit'); } catch { auditState = null; }
  renderAudit();
}

function renderAudit() {
  const s = auditState;
  const status = $('#auditStatus');
  if (!s) { status.textContent = ''; return; }
  $('#auditEnabled').checked = s.enabled;
  const input = $('#auditPath');
  input.placeholder = s.defaultPath || '';
  if (document.activeElement !== input) input.value = s.path || '';
  let text = t('hostWaiting');
  let cls = 'muted';
  if (!s.enabled) {
    text = t('auditOff');
  } else if (s.code) {
    text = t('audit_' + s.code) + (s.error || '');
    cls = 'bad';
  } else if (s.open) {
    text = t('auditOk', { p: s.path || s.defaultPath, r: fmtNum(s.read) });
    cls = 'good';
  }
  status.textContent = text;
  status.className = 'docker-status ' + cls;
}

async function configureAudit(change) {
  const body = { enabled: auditState?.enabled || false, path: auditState?.path || '', ...change };
  auditState = { ...(auditState || {}), ...body };
  renderAudit();
  try { auditState = await api('/api/audit', { method: 'PUT', body }); } catch (e) { toast(e.message); }
  renderAudit();
  setTimeout(loadAudit, 1500); // the file is read in the background
}

$('#auditEnabled').addEventListener('change', (ev) => configureAudit({ enabled: ev.target.checked }));
$('#auditPath').addEventListener('change', (ev) => configureAudit({ path: ev.target.value.trim() }));

/* ================= LogStream internal logs ================= */

let selfState = null;

async function loadSelfLog() {
  try { selfState = await api('/api/selflog'); } catch { selfState = null; }
  renderSelfLog();
}

function renderSelfLog() {
  const s = selfState;
  const status = $('#selfStatus');
  if (!s) { status.textContent = ''; return; }
  $('#selfEnabled').checked = s.enabled;
  $('#selfLevel').value = s.min_level || 'warning';
  $('#selfLevel').disabled = !s.enabled;
  const left = (s.dropped || 0) + (s.throttled || 0);
  status.textContent = s.enabled ? t('selfOk', { n: fmtNum(s.emitted || 0) }) + (left ? t('selfDropped', fmtNum(left)) : '') : t('selfOff');
  status.className = 'docker-status ' + (s.enabled ? 'good' : 'muted');
}

async function configureSelfLog(change) {
  const body = { enabled: selfState?.enabled ?? true, min_level: selfState?.min_level || 'warning', ...change };
  selfState = { ...(selfState || {}), ...body };
  renderSelfLog();
  try { selfState = await api('/api/selflog', { method: 'PUT', body }); } catch (e) { toast(e.message); }
  renderSelfLog();
}

$('#selfEnabled').addEventListener('change', (ev) => configureSelfLog({ enabled: ev.target.checked }));
$('#selfLevel').addEventListener('change', (ev) => configureSelfLog({ min_level: ev.target.value }));

/* ================= Docker source ================= */

let dockerState = null;
const dockerView = { showStopped: store.get('dockerShowStopped', '0') === '1', filter: '' };
const LOCK_ICON = '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>';

async function loadDocker() {
  try { dockerState = await api('/api/docker'); } catch (e) { dockerState = { enabled: true, connected: false, error: e.message }; }
  renderDocker();
}

// One label per choice key ("project/service"): replicas of a service share it.
function dockerChips() {
  const byKey = new Map();
  for (const c of (dockerState && dockerState.containers) || []) {
    const k = byKey.get(c.key);
    if (!k) {
      byKey.set(c.key, { ...c, names: [c.name], running: c.state === 'running', following: c.following });
    } else {
      k.names.push(c.name);
      k.running = k.running || c.state === 'running';
      k.following = k.following || c.following;
    }
  }
  return [...byKey.values()].sort((a, b) => a.key.localeCompare(b.key));
}

function chipHTML(c) {
  const state = c.locked ? t('stLocked') : !c.running ? t('stStopped') : c.following ? t('stFollowing') : c.enabled ? t('stSelected') : t('stIgnored');
  const title = [c.key, c.names.join(', '), c.image, state + (c.locked ? ` (${t('stLockedTitle')})` : '')].join('\n');
  const cls = ['chip', c.enabled && !c.locked ? 'on' : '', c.running ? '' : 'stopped', c.locked ? 'locked' : ''].filter(Boolean).join(' ');
  return `<button type="button" class="${cls}" style="--h:${hueOf(c.project || c.name)}" data-key="${esc(c.key)}"`
    + ` aria-pressed="${c.enabled && !c.locked}"${c.locked ? ' disabled' : ''} title="${esc(title)}">`
    + `${c.locked ? LOCK_ICON : ''}${esc(c.key)}</button>`;
}

function renderDocker() {
  const status = $('#dockerStatus');
  const body = $('#dockerBody');
  const d = dockerState;
  if (!d) { status.textContent = ''; body.hidden = true; return; }
  if (!d.enabled) {
    status.textContent = t('dockerOff');
    status.className = 'docker-status muted';
    body.hidden = true;
    return;
  }
  const cs = d.containers || [];
  if (!d.connected) {
    status.textContent = t('dockerErr') + (d.error || '');
    status.className = 'docker-status bad';
  } else {
    status.textContent = t('dockerOk', { h: d.host || 'docker', n: cs.length, f: cs.filter((c) => c.following).length });
    status.className = 'docker-status good';
  }
  body.hidden = !cs.length && !d.connected;
  $('#dockerDefault').checked = !!d.defaultEnabled;
  $('#dockerStopped').checked = dockerView.showStopped;

  const chips = dockerChips();
  $('#dockerStoppedLbl').textContent = t('dockerStopped', chips.filter((c) => !c.running && !c.locked).length);
  const q = dockerView.filter.toLowerCase();
  const visible = chips.filter((c) => (dockerView.showStopped || c.running) && (!q || c.key.toLowerCase().includes(q) || c.image.toLowerCase().includes(q)));
  const groups = [
    visible.filter((c) => c.enabled && !c.locked),
    visible.filter((c) => !c.enabled && !c.locked),
    visible.filter((c) => c.locked),
  ].filter((g) => g.length);
  $('#dockerList').innerHTML = groups.length
    ? groups.map((g) => g.map(chipHTML).join('')).join('<span class="chip-sep" aria-hidden="true"></span>')
    : `<span class="chip-empty">${esc(t('dockerNoMatch'))}</span>`;
}

async function configureDocker(body) {
  try {
    dockerState = await api('/api/docker', { method: 'PUT', body });
    renderDocker();
    setTimeout(loadDocker, 1500); // followers start or stop in the background
  } catch (e) { toast(e.message); loadDocker(); }
}

$('#dockerDefault').addEventListener('change', (ev) => configureDocker({ defaultEnabled: ev.target.checked }));
$('#dockerStopped').addEventListener('change', (ev) => {
  dockerView.showStopped = ev.target.checked;
  store.set('dockerShowStopped', dockerView.showStopped ? '1' : '0');
  renderDocker();
});
$('#dockerFilter').addEventListener('input', (ev) => { dockerView.filter = ev.target.value.trim(); renderDocker(); });
$('#dockerList').addEventListener('click', (ev) => {
  const chip = ev.target.closest('.chip[data-key]');
  if (!chip || chip.disabled) return;
  const key = chip.dataset.key;
  const on = !chip.classList.contains('on');
  // Immediate feedback, confirmed by the server answer.
  for (const c of (dockerState && dockerState.containers) || []) if (c.key === key) c.enabled = on;
  renderDocker();
  configureDocker({ containers: { [key]: on } });
});
for (const [id, on] of [['#dockerAll', true], ['#dockerNone', false]]) {
  $(id).addEventListener('click', () => {
    // Applies to the labels currently shown (filter and stopped option included).
    const q = dockerView.filter.toLowerCase();
    const containers = {};
    for (const c of dockerChips()) {
      if (c.locked || (!dockerView.showStopped && !c.running)) continue;
      if (q && !c.key.toLowerCase().includes(q) && !c.image.toLowerCase().includes(q)) continue;
      containers[c.key] = on;
    }
    configureDocker({ containers });
  });
}
// Keep the list fresh while the Sources tab is open.
setInterval(() => {
  if ($('#settingsDlg').open && !document.querySelector('[data-panel="sources"]').hidden) {
    loadDocker();
    loadHost();
    loadDmesg();
    loadAudit();
    loadSelfLog();
  }
}, 4000);

/* ================= Database statistics ================= */

// 1536 -> "1,5 Ko" / "1.5 KB" (binary multiples, the usual reading for disk sizes).
function fmtBytes(n) {
  const units = t('dbUnits');
  let i = 0;
  n = Number(n || 0);
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  const digits = i === 0 || n >= 100 ? 0 : 1;
  return `${n.toLocaleString(t('locale'), { maximumFractionDigits: digits })} ${units[i]}`;
}

const db = { stats: null, error: null, loading: false };

function renderDbStats() {
  const el = $('#dbStats');
  const s = db.stats;
  if (!s) {
    el.innerHTML = `<div class="db-msg${db.error ? ' bad' : ''}">${esc(db.error || t('dbLoading'))}</div>`;
    return;
  }
  const row = (k, v, sub) => `<dt>${esc(k)}</dt><dd><b>${esc(v)}</b>${sub ? ` <span class="muted">${esc(sub)}</span>` : ''}</dd>`;
  const day = (x) => fmtFull(x).replace(/[.,]\d{3}(?=\D*$)/, '');
  let html = row(t('dbRows'), fmtNum(s.rows));
  html += row(t('dbDisk'), fmtBytes(s.diskBytes), s.indexBytes ? t('dbIndex', fmtBytes(s.indexBytes)) : '');
  if (s.rawBytes) {
    const ratio = s.diskBytes ? (s.rawBytes / s.diskBytes).toLocaleString(t('locale'), { maximumFractionDigits: 1 }) : '';
    html += row(t('dbRaw'), fmtBytes(s.rawBytes), ratio ? t('dbRatio', ratio) : '');
  }
  if (s.oldest && s.newest) {
    const sub = [s.days ? t('dbDays', s.days) : '', s.retention ? t('dbRetention', s.retention) : ''].filter(Boolean).join(' · ');
    html += row(t('dbPeriod'), t('dbFromTo', { from: day(s.oldest), to: day(s.newest) }), sub);
  } else if (s.retention) {
    html += row(t('dbPeriod'), '—', t('dbRetention', s.retention));
  }
  if (s.rows) {
    html += row(t('dbRecent'), fmtNum(s.last24h), t('dbLastHour', fmtNum(s.last1h)));
    html += row(t('dbSources'), t('dbHostsApps', { h: fmtNum(s.hosts), hn: s.hosts, a: fmtNum(s.apps), an: s.apps }));
  }
  if (s.freeBytes) html += row(t('dbFree'), fmtBytes(s.freeBytes));
  if (!s.rows) html += `<div class="db-msg">${esc(t('dbEmpty'))}</div>`;
  if (s.queryError) html += `<div class="db-msg bad">${esc(t('dbQueryErr') + s.queryError)}</div>`;
  el.innerHTML = html;
}

async function loadDbStats(refresh = false) {
  if (db.loading) return;
  db.loading = true;
  $('#dbRefresh').disabled = true;
  try {
    db.stats = await api('/api/dbstats' + (refresh ? '?refresh=1' : ''));
    db.error = null;
  } catch (e) {
    db.stats = null;
    db.error = e.message;
  } finally {
    db.loading = false;
    $('#dbRefresh').disabled = false;
  }
  renderDbStats();
}
$('#dbRefresh').addEventListener('click', () => loadDbStats(true));

/* ================= Purge ================= */

const purge = { allowed: true, running: 0, error: null, code: null, wasRunning: false, timer: null };

function renderPurge() {
  const btn = $('#purgeBtn');
  const status = $('#purgeStatus');
  btn.disabled = !purge.allowed || purge.running > 0;
  let msg = '';
  let cls = 'muted';
  if (!purge.allowed) msg = t('purgeForbidden');
  else if (purge.code && I18N[lang]['err_' + purge.code]) { msg = t('err_' + purge.code); cls = 'bad'; }
  else if (purge.running > 0) msg = t('purgeRunning');
  else if (purge.wasRunning) { msg = t('purgeDone'); cls = 'good'; }
  status.textContent = msg;
  status.className = 'purge-status ' + cls;
  status.hidden = !msg;
}

async function loadPurgeStatus() {
  try {
    const s = await api('/api/purge');
    purge.allowed = s.allowed;
    purge.code = s.code || null;
    if (purge.running > 0 && !s.running) {
      purge.wasRunning = true;
      refresh();
      loadFacets();
      loadStats();
      loadDbStats(true);
    }
    purge.running = s.running || 0;
  } catch { /* shown on the next attempt */ }
  renderPurge();
  clearTimeout(purge.timer);
  if (purge.running > 0) purge.timer = setTimeout(loadPurgeStatus, 2000);
}

$('#purgeBtn').addEventListener('click', async () => {
  const answer = prompt(t('purgePrompt'));
  if (answer === null) return;
  try {
    await api('/api/purge', { method: 'POST', body: { confirm: answer.trim() } });
    toast(t('purgeStarted'));
    purge.running = 1;
    purge.wasRunning = false;
    state.rows = [];
    renderList();
    updateCount();
    loadPurgeStatus();
  } catch (e) {
    toast(e.message);
  }
});

/* ================= Tag management ================= */

const PALETTE = ['#6366f1', '#0ea5e9', '#14b8a6', '#a855f7', '#ec4899', '#eab308', '#64748b', '#f97316'];

// Ready-made tags, from the presets file (or the built-in presets.json):
// see docs/presets.md. A text is a string or an object per language.
const pickText = (x) => (typeof x === 'string' ? x : (x?.[lang] ?? x?.en ?? Object.values(x || {})[0] ?? ''));

async function loadPresets() {
  try {
    const res = await api('/api/presets');
    state.presets = res.groups || [];
    if (res.error) toast(t('presetsFileErr', res.file) + res.error);
  } catch (e) { toast(e.message); }
  renderPresets();
}

function renderPresets() {
  $('#presetTags').innerHTML = `<option value="">${esc(t('presetPick'))}</option>`
    + state.presets.map((g) => `<optgroup label="${esc(pickText(g.group))}">`
      + g.presets.map((p) => `<option value="${esc(p.id)}">${esc(pickText(p.name))}</option>`).join('') + '</optgroup>').join('');
}

async function loadTags() {
  try { state.tags = await api('/api/tags'); } catch (e) { toast(t('tagsLoadErr') + e.message); }
  compileMatchers();
}

function tagRowHTML(tag) {
  const opt = (field, label, title) =>
    `<label class="opt" title="${esc(title)}"><input type="checkbox" data-f="${field}"${tag[field] ? ' checked' : ''}>${esc(label)}</label>`;
  return `<div class="tag-row${tag.enabled ? '' : ' off'}" data-id="${esc(tag.id)}">
    <span class="tag-code" title="${esc(t('tagCodeTitle'))}">${esc(tag.code || '··')}</span>
    <input type="color" value="${esc(tag.color)}" data-f="color" aria-label="${esc(t('colorAria'))}">
    <input type="text" value="${esc(tag.pattern)}" data-f="pattern" placeholder="${esc(t('keyword'))}" spellcheck="false" aria-label="${esc(t('keyword'))}">
    <span class="preview" title="${esc(tag.pattern)}"><mark class="tag" style="${tagStyle(tag.color)}">${esc(tag.label || tag.pattern || t('preview'))}</mark></span>
    <div class="opts">
      ${opt('wholeWord', t('optWhole'), t('optWholeTitle'))}
      ${opt('caseSensitive', 'Aa', t('optCaseTitle'))}
      ${opt('regex', '.*', t('optRegexTitle'))}
      ${opt('enabled', t('optActive'), t('optActiveTitle'))}
    </div>
    <button class="icon-btn danger" type="button" data-del title="${esc(t('del'))}" aria-label="${esc(t('del'))}">
      <svg viewBox="0 0 24 24"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/></svg>
    </button>
    <div class="err"></div>
  </div>`;
}

function renderTagList() {
  $('#tagList').innerHTML = state.tags.map(tagRowHTML).join('') || `<p class="muted small">${esc(t('noTags'))}</p>`;
  filterTagList();
}

// Settings > Pattern: shows only the tags whose keyword, label or code contains the filter.
function filterTagList() {
  const q = $('#tagFilter').value.trim().toLowerCase();
  let shown = 0;
  for (const row of $('#tagList').querySelectorAll('.tag-row')) {
    const tag = state.tags.find((x) => x.id === row.dataset.id);
    const hit = !q || !tag || [tag.pattern, tag.label, tag.code].some((v) => String(v || '').toLowerCase().includes(q));
    row.hidden = !hit;
    if (hit) shown++;
  }
  let none = $('#tagList').querySelector('.tag-none');
  if (q && !shown && state.tags.length) {
    if (!none) {
      none = document.createElement('p');
      none.className = 'muted small tag-none';
      $('#tagList').append(none);
    }
    none.textContent = t('noTagMatch');
  } else if (none) none.remove();
  $('#tagCount').textContent = state.tags.length ? t('tagCount', { shown, total: state.tags.length }) : '';
}
$('#tagFilter').addEventListener('input', filterTagList);

const saveTimers = {};
function scheduleSave(tag, rowEl) {
  clearTimeout(saveTimers[tag.id]);
  saveTimers[tag.id] = setTimeout(async () => {
    const errEl = rowEl.querySelector('.err');
    try {
      await api('/api/tags/' + encodeURIComponent(tag.id), { method: 'PUT', body: tag });
      errEl.textContent = '';
    } catch (e) {
      errEl.textContent = e.message;
    }
  }, 400);
}

const applyTags = debounce(() => { compileMatchers(); rehighlight(); }, 150);

$('#tagList').addEventListener('input', (ev) => {
  const rowEl = ev.target.closest('.tag-row');
  const field = ev.target.dataset.f;
  if (!rowEl || !field) return;
  const tag = state.tags.find((x) => x.id === rowEl.dataset.id);
  if (!tag) return;
  tag[field] = ev.target.type === 'checkbox' ? ev.target.checked : ev.target.value;
  rowEl.classList.toggle('off', !tag.enabled);
  const mark = rowEl.querySelector('.preview mark');
  mark.textContent = tag.label || tag.pattern || t('preview');
  mark.setAttribute('style', tagStyle(tag.color));
  scheduleSave(tag, rowEl);
  applyTags();
});

$('#tagList').addEventListener('click', async (ev) => {
  const del = ev.target.closest('[data-del]');
  if (!del) return;
  const rowEl = del.closest('.tag-row');
  const tag = state.tags.find((x) => x.id === rowEl.dataset.id);
  if (!tag || !confirm(t('confirmDelete', tag.pattern))) return;
  try {
    await api('/api/tags/' + encodeURIComponent(tag.id), { method: 'DELETE' });
    state.tags = state.tags.filter((x) => x !== tag);
    renderTagList();
    applyTags();
  } catch (e) { toast(e.message); }
});

$('#addTag').addEventListener('click', async () => {
  const color = PALETTE[state.tags.length % PALETTE.length];
  try {
    const tag = await api('/api/tags', { method: 'POST', body: { pattern: t('newTag'), color, wholeWord: true, enabled: true } });
    state.tags.push(tag);
    $('#tagFilter').value = '';
    renderTagList();
    applyTags();
    const input = $('#tagList').lastElementChild.querySelector('input[type="text"]');
    input.focus();
    input.select();
  } catch (e) { toast(e.message); }
});

// Adds a preset group, skipping tags whose pattern is already in the list.
$('#presetTags').addEventListener('change', async (ev) => {
  const preset = state.presets.flatMap((g) => g.presets).find((p) => p.id === ev.target.value);
  ev.target.value = '';
  if (!preset) return;
  let added = 0;
  try {
    for (const p of preset.tags) {
      if (state.tags.some((x) => x.pattern === p.pattern)) continue;
      const tag = { ...p, label: pickText(p.label), regex: p.regex ?? true, enabled: true };
      state.tags.push(await api('/api/tags', { method: 'POST', body: tag }));
      added++;
    }
  } catch (e) { toast(e.message); }
  renderTagList();
  applyTags();
  toast(t('presetAdded', added));
});

$('#resetTags').addEventListener('click', async () => {
  if (!confirm(t('confirmReset'))) return;
  try {
    state.tags = await api('/api/tags/reset', { method: 'POST' });
    renderTagList();
    applyTags();
  } catch (e) { toast(e.message); }
});

$('#settingsBtn').addEventListener('click', () => {
  renderTimeSettings();
  renderTagList();
  loadPresets();
  renderInterface();
  loadPurgeStatus();
  loadSyslog();
  loadHost();
  loadDmesg();
  loadAudit();
  loadSelfLog();
  loadDocker();
  renderDbStats();
  loadDbStats();
  showSettingsTab(store.get('settingsTab', 'locale'));
  $('#settingsDlg').showModal();
});
$('#closeSettings').addEventListener('click', () => $('#settingsDlg').close());
$('#settingsDlg').addEventListener('click', (ev) => { if (ev.target === ev.currentTarget) ev.currentTarget.close(); });

/* ================= Startup ================= */

{
  const tz = store.get('tz', '');
  timePrefs.tz = validTimeZone(tz) ? tz : '';
  const fmt = store.get('timefmt', DEFAULT_TIME_FMT);
  timePrefs.fmt = TIME_FORMATS.some((f) => f.id === fmt) ? fmt : DEFAULT_TIME_FMT;
}
applyLogFont(store.get('logFont', 'system'));
applyLogSize(store.get('logSize', 'medium'));
applyLogDensity(store.get('logDensity', 'normal'));
$('#authWarnClose').addEventListener('click', () => {
  $('#authWarn').hidden = true;
  store.set('authWarnHidden', '1');
});

applyLang();
$('#range').value = store.get('range', '1h');
if (!$('#range').value) $('#range').value = '1h';
$('#severity').value = store.get('severity', '');
try { // Time range mode is kept across reloads
  const z = JSON.parse(store.get('period', '') || 'null');
  if (z && z.to > z.from) state.zoom = { from: z.from, to: z.to };
} catch { /* stream mode */ }
renderPeriod();

// Without a login, warn that the UI is open to everyone. With a login (local or OIDC),
// show who is logged in and the log out button, and lock the admin settings of a
// read-only account.
fetch('/auth/me').then((res) => (res.ok ? res.json() : null)).then((me) => {
  if (!me) return;
  if (me.mode === 'none' && store.get('authWarnHidden') !== '1') $('#authWarn').hidden = false;
  const who = $('#whoami');
  if (me.mode === 'none') {
    who.classList.add('none');
    who.querySelector('.who-name').textContent = '(no auth)';
    who.hidden = false;
  } else if (me.user) {
    who.querySelector('.who-name').textContent = me.user;
    who.title = me.user;
    const badge = who.querySelector('.who-badge');
    badge.textContent = me.mode === 'oidc' ? 'OIDC' : 'local';
    badge.classList.add(me.mode);
    who.hidden = false;
  }
  if (me.role === 'viewer') {
    document.body.classList.add('read-only');
    for (const p of document.querySelectorAll('[data-panel="filters"], [data-panel="sources"], [data-panel="data"]')) {
      for (const s of p.querySelectorAll('.set-section')) s.inert = true;
      const note = document.createElement('p');
      note.className = 'ro-note';
      note.dataset.i18n = 'readOnlyNote';
      note.textContent = t('readOnlyNote');
      p.prepend(note);
    }
  }
  if (!me.user) return;
  const btn = $('#logoutBtn');
  btn.hidden = false;
  btn.dataset.user = me.user;
  btn.title = `${t('logout')} (${me.user})`;
}).catch(() => {});

(async () => {
  await loadTags();
  loadFacets();
  loadStats();
  refresh();
  setInterval(loadStats, 5000);
  setInterval(loadFacets, 60000);
})();
