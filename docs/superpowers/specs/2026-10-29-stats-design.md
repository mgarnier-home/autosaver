# Autosaver 1.2.0 — stats pour homepage, spec de conception

Date : 29/10/2026.

## Objectif

Afficher sur la carte Autosaver de homepage l'état des sauvegardes : ancienneté du dernier succès,
échecs, durée, taille de l'archive et progression d'une sauvegarde en cours.

**Critères de réussite :**

- une sauvegarde qui ne tourne plus, ou qui échoue, se voit d'un coup d'œil (chip `warn` ou `bad`) ;
- l'historique survit aux redémarrages et aux déploiements du conteneur ;
- une sauvegarde interrompue par un redémarrage laisse une trace dans l'historique ;
- homepage reste seul juge de la présentation : textes, seuils et couleurs ne demandent pas de
  nouvelle version d'autosaver.

## Hypothèses

- Une sauvegarde est lancée une fois par jour (appel de `/run`).
- `/last` garde ses réponses actuelles : l'ancienne version de homepage continue de fonctionner
  jusqu'à sa mise à jour. Autosaver 1.2.0 est déployé avant la nouvelle version de homepage.

## Hors périmètre

- Grafana / InfluxDB, format Prometheus.
- L'espace libre sur la destination distante.
- Toute nouvelle variable d'environnement ou clé de configuration.

## API d'autosaver : `GET /stats`

JSON, dates en UTC au format RFC 3339, tailles en octets, durées en secondes entières.

```json
{
  "running": true,
  "progress": { "step": "upload", "percent": 43.2, "startedAt": "2026-10-30T03:00:02Z" },
  "lastSuccessAt": "2026-10-29T04:12:40Z",
  "consecutiveFailures": 0,
  "history": [
    {
      "startedAt": "2026-10-29T03:00:01Z",
      "finishedAt": "2026-10-29T04:12:40Z",
      "success": true,
      "error": "",
      "durationSeconds": 4359,
      "steps": { "zip": 1510, "encrypt": 640, "upload": 2209, "cleanup": 0 },
      "sourceBytes": 61203456789,
      "archiveBytes": 33912345678
    }
  ]
}
```

- `progress` vaut `null` hors sauvegarde. `step` vaut `zip`, `encrypt`, `upload` ou `cleanup` ;
  `startedAt` est le début de la sauvegarde, pas de l'étape.
- `lastSuccessAt` vaut `null` tant qu'aucune sauvegarde n'a réussi.
- `history` garde les 30 dernières exécutions, de la plus ancienne à la plus récente.
- Une exécution en échec ne contient dans `steps` que les étapes atteintes ; une taille encore
  inconnue vaut `0`. `error` est le message d'erreur, vide en cas de succès.
- `lastSuccessAt` et `consecutiveFailures` sont tenus à part de `history`, pour rester justes après
  plus de 30 échecs consécutifs.
- Le temps d'envoi du mail du mot de passe est compté dans l'étape `encrypt`.

## Fichier d'état

- Chemin déduit de `CONFIG_FILE_PATH` en remplaçant l'extension par `.stats.json` :
  `/config/config-atlas.yml` → `/config/config-atlas.stats.json`.
- Contenu : `lastSuccessAt`, `consecutiveFailures`, `history`, et `currentStartedAt`, le début de la
  sauvegarde en cours (absent hors sauvegarde). La progression n'est pas écrite.
- Écrit au début et à la fin de chaque exécution, dans un fichier temporaire du même dossier puis
  renommé : un arrêt brutal ne laisse jamais un JSON tronqué. Une erreur d'écriture est journalisée
  et n'arrête ni la sauvegarde ni le service.
- Au démarrage :
  - fichier absent : état vide ;
  - fichier illisible : état vide, avec un avertissement dans les logs ;
  - `currentStartedAt` présent : la sauvegarde a été interrompue. Elle est ajoutée à l'historique
    comme un échec « interrompue par un redémarrage », terminé à l'heure du démarrage, et
    `consecutiveFailures` augmente de 1.

## Découpage du code d'autosaver

- Paquet `stats`, sans dépendance au reste d'autosaver : l'état, sa persistance et l'API d'écriture
  utilisée par la sauvegarde (`Start`, `Step`, `Progress`, `SetSourceBytes`, `SetArchiveBytes`,
  `Finish`) et de lecture (`Snapshot`). Un mutex protège l'état, lu par le serveur HTTP pendant que
  la sauvegarde l'écrit. L'horloge est injectée pour les tests.
- `backup` et `external` reçoivent le `*stats.Store` et signalent chaque étape et sa progression.
  `zipFolder` renvoie la taille des données source qu'il calcule déjà ; la taille de l'archive vient
  d'`os.Stat` sur le `.gpg`.
- `server` sert `GET /stats`. `/last` ne change pas.

## Homepage : intégration `autosaver`

L'intégration lit `/stats` au lieu de `/last`. Chips, dans l'ordre :

1. **Ancienneté du dernier succès** : `dernier succès il y a 5 h` (`il y a N min` sous 1 h,
   `il y a N h` sous 48 h, `il y a N j` au-delà). `neutral` jusqu'à 36 h, `warn` au-delà, `bad`
   au-delà de 3 jours. Info-bulle : `fin le 29/10/2026 à 05:12`, en heure de Paris.
   - Aucun succès mais un historique non vide : `bad` `aucune sauvegarde réussie`.
   - Historique vide (première installation) : `neutral` `aucune sauvegarde enregistrée`.
2. **Échecs**, si `consecutiveFailures` > 0 : `bad`, `dernière sauvegarde échouée` pour 1,
   `N échecs consécutifs` au-delà. Info-bulle : `error` de la dernière exécution.
3. **Durée ou progression**
   - En cours : `en cours · envoi 43 %` (étapes : compression, chiffrement, envoi, nettoyage).
     Info-bulle : `démarrée à 05:00`, en heure de Paris.
   - Sinon, s'il existe un succès : durée de la dernière sauvegarde réussie (`45 s`, `37 min`,
     `1 h 12 min`), série des durées des succès de l'historique. Info-bulle :
     `compression 25 min · chiffrement 11 min · envoi 37 min (15,3 Mo/s)`.
4. **Taille de l'archive**, s'il existe un succès : `31,6 Go`, série des tailles des succès.
   Info-bulle : `données : 57,0 Go · archive : 55,4 % des données`. `warn` si l'archive pèse
   moins de la moitié de la précédente réussie, et l'info-bulle commence alors par
   `moitié moins que la précédente (64,1 Go)`.

Erreurs : un statut autre que 200 ou un JSON illisible donnent `stats indisponibles`, comme pour les
autres intégrations. Pas de repli sur `/last`.

## Tests

- Autosaver, paquet `stats` : limite de 30 exécutions ; `lastSuccessAt` et `consecutiveFailures`
  après une suite de succès et d'échecs ; durée de chaque étape ; aller-retour par le fichier ;
  fichier absent ou corrompu ; sauvegarde interrompue détectée au redémarrage.
- Autosaver, `server` : `GET /stats` avec `httptest`.
- Homepage : chaque chip et chaque seuil sur des réponses `/stats` de test, horloge fixée via `now` ;
  les deux nouvelles fonctions de mise en forme.

## Livraison

1. PR autosaver, puis tag `v1.2.0` (image `mgarnier11/autosaver:1.2.0`).
2. Mise à jour de l'image dans les fichiers compose, déploiement.
3. PR homepage.
