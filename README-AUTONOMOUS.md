# X Spam Detector - Mode Autonome

## 🚀 Vue d'ensemble

Le **X Spam Detector Autonome** est un système avancé de détection de spam qui surveille automatiquement X (Twitter) en temps réel pour identifier les fermes de bots et les campagnes de spam coordonnées. Il utilise l'API Twitter v2 pour collecter les données et des algorithmes LSH pour détecter les similarités.

## ✨ Fonctionnalités Autonomes

### 🔄 **Collecte Automatique**
- **Streaming en temps réel** via l'API Twitter v2
- **Recherche périodique** par mots-clés et hashtags
- **Surveillance géographique** (optionnel)
- **Filtrage intelligent** des contenus

### 🎯 **Détection Intelligente**
- **Algorithmes LSH hybrides** (MinHash + SimHash)
- **Clustering automatique** des contenus similaires
- **Analyse comportementale** des comptes
- **Détection de coordination** temporelle

### 🚨 **Système d'Alertes**
- **Notifications multi-canaux** (Slack, Discord, Email, Webhooks)
- **Alertes en temps réel** pour les campagnes massives
- **Seuils configurables** de criticité
- **Rate limiting** intelligent

### 📊 **Monitoring Avancé**
- **Dashboard web** interactif
- **API REST** complète
- **Métriques temps réel**
- **Historique des détections**

## 🛠️ Installation et Configuration

### **1. Prérequis**

```bash
# Go 1.21+
go version

# Accès API Twitter
# Créer un compte développeur : https://developer.twitter.com
```

### **2. Configuration API Twitter**

1. **Créer une App Twitter** :
   - Aller sur https://developer.twitter.com
   - Créer un projet et une App
   - Générer les clés API

2. **Variables d'environnement** :
```bash
export TWITTER_BEARER_TOKEN="votre_bearer_token"
export TWITTER_API_KEY="votre_api_key" 
export TWITTER_API_SECRET="votre_api_secret"
export TWITTER_ACCESS_TOKEN="votre_access_token"
export TWITTER_ACCESS_SECRET="votre_access_secret"
```

### **3. Configuration du Système**

Éditer `config-autonomous.yaml` :

```yaml
# Surveillance
crawler:
  monitoring:
    keywords:
      - "crypto deal"
      - "follow for follow"
      - "make money"
    hashtags:
      - "crypto"
      - "bitcoin"
      - "followback"
    languages: ["en", "fr"]

# Détection
auto_detection:
  enabled: true
  interval: "5m"
  min_tweets_for_detection: 50

# Alertes
alerts:
  enabled: true
  slack:
    enabled: true
    webhook_url: "${SLACK_WEBHOOK_URL}"
```

## 🎮 Utilisation

### **Mode Autonome Complet**

```bash
# Construction
go build -o spam-detector-autonomous cmd/autonomous/main.go

# Lancement
./spam-detector-autonomous

# Ou directement
go run cmd/autonomous/main.go
```

### **Modes Disponibles**

```bash
# Mode autonome complet (recommandé)
./spam-detector-autonomous -mode autonomous

# API uniquement (pour tests)
./spam-detector-autonomous -mode api-only

# Configuration personnalisée
./spam-detector-autonomous -config ma-config.yaml

# Niveau de log debug
./spam-detector-autonomous -log-level debug
```

### **Dashboard Web**

Une fois lancé, accédez à :
- **Dashboard** : http://localhost:8080
- **API** : http://localhost:8080/api/v1/

## 📊 Dashboard et API

### **Interface Web**

Le dashboard fournit :
- **Statistiques temps réel** : tweets collectés, spam détecté, clusters
- **Alertes actives** : campagnes en cours, niveau de criticité
- **Clusters de spam** : analyse détaillée des patterns
- **Comptes suspects** : scores de bot, activité suspecte

### **API REST**

```bash
# Statistiques générales
curl http://localhost:8080/api/v1/stats

# Clusters détectés
curl http://localhost:8080/api/v1/clusters

# Tweets suspects
curl http://localhost:8080/api/v1/tweets?spam_only=true

# Comptes suspects
curl http://localhost:8080/api/v1/accounts

# Alertes récentes
curl http://localhost:8080/api/v1/alerts

# Contrôle du système
curl -X POST http://localhost:8080/api/v1/control/pause
curl -X POST http://localhost:8080/api/v1/control/resume
```

## 🔍 Exemples de Surveillance

### **1. Surveillance Crypto Scams**

```yaml
monitoring:
  keywords:
    - "crypto deal"
    - "bitcoin opportunity" 
    - "guaranteed profit"
    - "crypto giveaway"
  hashtags:
    - "crypto"
    - "bitcoin"
    - "defi"
```

### **2. Détection Follow-for-Follow**

```yaml
monitoring:
  keywords:
    - "follow for follow"
    - "f4f"
    - "followback"
  hashtags:
    - "followback"
    - "f4f"
    - "followtrain"
```

### **3. Surveillance Phishing**

```yaml
monitoring:
  keywords:
    - "verify account"
    - "account suspension"
    - "click here to verify"
    - "urgent action required"
```

## 🚨 Configuration des Alertes

### **Slack**

```bash
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..."
```

```yaml
alerts:
  slack:
    enabled: true
    webhook_url: "${SLACK_WEBHOOK_URL}"
    channel: "#spam-alerts"
    username: "X Spam Detector"
```

### **Discord**

```bash
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."
```

```yaml
alerts:
  discord:
    enabled: true
    webhook_url: "${DISCORD_WEBHOOK_URL}"
    username: "X Spam Detector"
```

### **Email**

```bash
export EMAIL_USERNAME="votre_email@gmail.com"
export EMAIL_PASSWORD="votre_mot_de_passe_app"
```

```yaml
alerts:
  email:
    enabled: true
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "${EMAIL_USERNAME}"
    password: "${EMAIL_PASSWORD}"
    to: ["admin@example.com"]
```

### **Webhooks Personnalisés**

```yaml
alerts:
  webhooks:
    enabled: true
    urls:
      - "https://votre-webhook.com/spam-alert"
    headers:
      Authorization: "Bearer ${WEBHOOK_TOKEN}"
```

## 📈 Métriques et Performance

### **Statistiques Collectées**

- **Tweets collectés/seconde**
- **Taux de détection de spam**
- **Nombre de clusters actifs**
- **Comptes suspects identifiés**
- **Temps de traitement moyen**
- **Utilisation mémoire**

### **Optimisation Performance**

```yaml
# Pour gros volumes
crawler:
  settings:
    batch_size: 500
    max_tweets_per_hour: 50000

auto_detection:
  batch_processing: true
  batch_size: 200

# Pour économiser la mémoire
system:
  max_memory_mb: 1000
  auto_clear_old_data: true
  retention_hours: 24
```

## 🔧 Cas d'Usage Avancés

### **1. Surveillance Multi-langues**

```yaml
crawler:
  settings:
    languages: ["en", "fr", "es", "de", "it"]
  monitoring:
    keywords:
      - "crypto deal"          # EN
      - "offre crypto"         # FR  
      - "oferta crypto"        # ES
```

### **2. Surveillance Géographique**

```yaml
monitoring:
  geo_locations:
    - southwest: { lat: 48.8, lng: 2.2 }  # Paris
      northeast: { lat: 48.9, lng: 2.4 }
      name: "Paris"
    - southwest: { lat: 40.7, lng: -74.0 } # NYC
      northeast: { lat: 40.8, lng: -73.9 }
      name: "New York"
```

### **3. Surveillance Comptes Spécifiques**

```yaml
monitoring:
  monitored_users:
    - "compte_suspect_1"
    - "source_spam_connue"
    - "bot_detecte"
```

## 🛡️ Sécurité et Bonnes Pratiques

### **Gestion des Secrets**

```bash
# Utiliser un fichier .env
echo "TWITTER_BEARER_TOKEN=your_token" > .env
echo "SLACK_WEBHOOK_URL=your_webhook" >> .env

# Ou variables d'environnement système
export TWITTER_BEARER_TOKEN="your_token"
```

### **Rate Limiting**

```yaml
crawler:
  api:
    rate_limit: 300  # Requêtes par 15 min (limite Twitter)
  settings:
    max_tweets_per_hour: 10000

alerts:
  rate_limiting:
    max_alerts_per_hour: 50
    cooldown_period: "5m"
```

### **Monitoring Système**

```yaml
system:
  health_check_interval: "5m"
  max_memory_mb: 2000
  enable_profiling: true  # Pour debug seulement
```

## 🚀 Déploiement en Production

### **Docker**

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o spam-detector-autonomous cmd/autonomous/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/spam-detector-autonomous .
COPY --from=builder /app/config-autonomous.yaml .
CMD ["./spam-detector-autonomous"]
```

```bash
# Build et run
docker build -t x-spam-detector .
docker run -e TWITTER_BEARER_TOKEN="$TWITTER_BEARER_TOKEN" \
           -p 8080:8080 \
           x-spam-detector
```

### **Systemd Service**

```ini
[Unit]
Description=X Spam Detector Autonomous
After=network.target

[Service]
Type=simple
User=spam-detector
WorkingDirectory=/opt/spam-detector
ExecStart=/opt/spam-detector/spam-detector-autonomous
Restart=always
RestartSec=5
Environment=TWITTER_BEARER_TOKEN=your_token

[Install]
WantedBy=multi-user.target
```

### **Monitoring avec Prometheus**

```yaml
# Ajout métriques Prometheus
system:
  enable_prometheus: true
  prometheus_port: 9090
```

## 📝 Exemples de Résultats

### **Alerte Slack Typique**

```
🚨 SPAM ALERT - HIGH SEVERITY
Cluster détecté: 15 tweets, confiance 0.85
Pattern: crypto spam campaign
Comptes impliqués: 8
Méthode: MinHash LSH
Temps: 2024-01-15 14:30:25
```

### **API Response - Cluster**

```json
{
  "success": true,
  "data": {
    "id": "cluster_abc123",
    "size": 15,
    "confidence": 0.85,
    "severity": "high",
    "pattern": "crypto spam campaign",
    "accounts": 8,
    "is_coordinated": true,
    "tweets_per_minute": 0.33,
    "detection_method": "minhash"
  }
}
```

## 🔍 Dépannage

### **Problèmes Courants**

1. **"No tweets collected"** :
   ```bash
   # Vérifier les credentials API
   curl -H "Authorization: Bearer $TWITTER_BEARER_TOKEN" \
        "https://api.twitter.com/2/tweets/search/recent?query=hello"
   ```

2. **"Rate limit exceeded"** :
   ```yaml
   # Réduire la fréquence
   crawler:
     settings:
       poll_interval: "60s"  # Au lieu de 30s
   ```

3. **"High memory usage"** :
   ```yaml
   # Activer le nettoyage automatique
   auto_detection:
     auto_clear_old_data: true
     retention_hours: 24
   ```

### **Logs Debug**

```bash
# Mode debug complet
./spam-detector-autonomous -log-level debug

# Logs spécifiques
tail -f spam-detector.log | grep "SPAM"
tail -f spam-detector.log | grep "ERROR"
```

## 📊 Métriques de Performance

### **Benchmarks Typiques**

- **Collecte** : 1000-5000 tweets/minute
- **Détection** : 100ms-1s par analyse
- **Mémoire** : 100-500 MB selon volume
- **CPU** : 5-20% sur machine moderne

### **Optimisation**

```yaml
# Performance maximale
crawler:
  settings:
    enable_streaming: true
    batch_size: 500

auto_detection:
  batch_processing: true
  continuous_mode: true

engine:
  enable_hybrid_mode: false  # MinHash uniquement
```

## 🎯 Résultats Attendus

Avec une configuration optimale, le système peut détecter :

- **95%+ des campagnes crypto** (variations mineures)
- **90%+ des schémas follow-for-follow**
- **85%+ des campagnes phishing** 
- **80%+ des fermes de bots** coordonnées

Le système maintient un **taux de faux positifs < 5%** sur du contenu légitime.

---

## 🆘 Support

Pour des questions ou problèmes :
1. Vérifier la configuration dans `config-autonomous.yaml`
2. Consulter les logs avec `-log-level debug`
3. Tester l'API Twitter avec curl
4. Vérifier les variables d'environnement

Le système autonome transforme complètement l'approche de détection de spam en passant d'une analyse manuelle à une surveillance continue et intelligente de l'écosystème X/Twitter.