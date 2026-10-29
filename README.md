# autosaver

Service de sauvegarde : zippe et chiffre un dossier, puis copie l'archive en local et/ou sur un
hôte distant en SFTP. Une sauvegarde se lance avec `GET /run` ; `GET /last` donne le résultat de la
dernière exécution.

## Configuration

Variables d'environnement :

- `CONFIG_FILE_PATH` : chemin du fichier YAML de configuration (défaut `./data/config.yaml`).
- `SERVER_PORT` : port HTTP (défaut `8080`).
- `SSH_PRIVATE_KEY` : clé privée pour la copie SFTP.
- `NTFY_SERVER`, `NTFY_TOPIC` : notifications ntfy.
- `ENV_FILE_PATH` : fichier `.env` chargé au démarrage (dev).

## Développement

```sh
task start      # build + lancement avec /mnt/dev/autosaver/.env
task test
```

Les tests tournent sur chaque PR vers `main`. L'image `mgarnier11/go-autosaver` est publiée en
poussant un tag `vX.Y.Z` sur un commit de `main` (image taguée `X.Y.Z` et `latest`).
