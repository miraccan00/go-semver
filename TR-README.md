# go-semver

[Conventional Commits](https://www.conventionalcommits.org/) kurallarını okuyarak Git repolarında semantic versioning'i otomatikleştiren, CI/CD pipeline'ları için zengin JSON metadata üreten hafif bir CLI aracı.

## Kurulum & Derleme

```bash
go build -o new-semver ./cmd/new-semver
```

## Kullanım

Repo kökünde çalıştırın:

```bash
./new-semver
```

Stdout'a JSON metadata basar ve `VERSION` dosyasını günceller.

## Versiyon Belirleme Mantığı

```
VERSION dosyası var?
  ├── Evet → dosyadaki versiyonu oku
  └── Hayır → GitVersion.yml içindeki next-version: alanını kullan

En az bir commit var mı?
  ├── Evet → son commit mesajını parse et, versiyon artır, VERSION dosyasına yaz
  └── Hayır → GitVersion.yml'deki next-version ile devam et (bump yok)
```

### Conventional Commits → Versiyon Artışı

| Commit Prefix | Örnek | Etki |
|---|---|---|
| `fix:` | `fix: null pointer in auth` | PATCH artışı |
| `feat:` | `feat: add retry logic` | MINOR artışı |
| `feat!:` veya `fix!:` | `feat!: drop v1 API` | MAJOR artışı |
| `BREAKING CHANGE:` | `BREAKING CHANGE: schema renamed` | MAJOR artışı |
| Diğerleri (`chore:`, `docs:`, vb.) | `chore: update deps` | Artış yok |

> Eşleştirme büyük/küçük harf duyarsızdır.

## JSON Çıktısı

```json
{
    "Major": 1,
    "Minor": 3,
    "Patch": 0,
    "MajorMinorPatch": "1.3.0",
    "SemVer": "1.3.0",
    "FullSemVer": "1.3.0+42",
    "InformationalVersion": "1.3.0+42.Branch.main.Sha.abc1234...",
    "BranchName": "main",
    "EscapedBranchName": "main",
    "Sha": "abc1234...",
    "ShortSha": "abc1234",
    "CommitsSinceVersionSource": 42,
    "CommitDate": "2026-04-10",
    "UncommittedChanges": 0,
    "BuildMetaData": 42,
    "AssemblySemVer": "1.3.0.0",
    "AssemblySemFileVer": "1.3.0.0"
}
```

## Dosyalar

| Dosya | Açıklama |
|---|---|
| `VERSION` | `X.Y.Z` formatında aktif versiyon — araç tarafından yönetilir, bu dosyayı commit'leyin |
| `GitVersion.yml` | `VERSION` dosyası yoksa `next-version:` alanı fallback olarak kullanılır |

### GitVersion.yml

```yaml
next-version: 0.0.1       # VERSION dosyası yoksa kullanılır
mode: ContinuousDeployment
```

## Docker

`main`'e her push'ta image otomatik olarak GitHub Container Registry'ye yayınlanır:

```bash
docker pull ghcr.io/miraccan00/go-semver:latest
```

Yerel reponuzda çalıştırmak için:

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/miraccan00/go-semver:latest
```

> Container içinde `git` kurulu gelir. Aracın `.git/`, `VERSION` ve `GitVersion.yml` dosyalarını okuyabilmesi için repo kökünüzü `/workspace` olarak mount edin.

---

## CI/CD Entegrasyonu

### Hızlı Başlangıç (tüm CI sistemleri için)

Versiyon pipeline çalışmaları arasında kaybolmasın diye repo kökünüze bir `VERSION` dosyası ekleyin:

```bash
echo "0.1.0" > VERSION
git add VERSION
git commit -m "chore: add initial VERSION file"
```

Ardından pipeline'ın başında go-semver'ı çalıştırarak versiyonu artırın ve çıktıyı yakalayın:

```bash
VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
  ghcr.io/miraccan00/go-semver:latest)

# Versiyon string'ini ayıkla
APP_VERSION=$(echo "$VERSION_JSON" | jq -r '.MajorMinorPatch')
echo "Derlenen versiyon: $APP_VERSION"
```

---

### GitHub Actions

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0          # zorunlu — go-semver git log'u okur

      - name: go-semver çalıştır
        id: semver
        run: |
          VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
            ghcr.io/miraccan00/go-semver:latest)
          echo "json=$VERSION_JSON" >> $GITHUB_OUTPUT
          echo "version=$(echo $VERSION_JSON | jq -r '.MajorMinorPatch')" >> $GITHUB_OUTPUT

      - name: Versiyonu kullan
        run: |
          echo "Uygulama versiyonu: ${{ steps.semver.outputs.version }}"
          docker build -t myapp:${{ steps.semver.outputs.version }} .
```

> **Önemli:** `fetch-depth: 0` kullanın — varsayılan shallow clone `git log`'u ve commit sayımını bozar.

---

### GitLab CI

```yaml
variables:
  DOCKER_DRIVER: overlay2

semver:
  image: ghcr.io/miraccan00/go-semver:latest
  stage: .pre
  script:
    - VERSION_JSON=$(new-semver)
    - echo "APP_VERSION=$(echo $VERSION_JSON | jq -r '.MajorMinorPatch')" >> build.env
    - echo "SHORT_SHA=$(echo $VERSION_JSON | jq -r '.ShortSha')" >> build.env
  artifacts:
    reports:
      dotenv: build.env   # APP_VERSION'ı sonraki job'lara aktarır

build:
  stage: build
  needs: [semver]
  script:
    - docker build -t myapp:$APP_VERSION .
```

> **Önemli:** GitLab projenizin CI/CD değişkenlerine `GIT_DEPTH: 0` ekleyin ya da job içine yazın; aksi halde tam git geçmişi alınamaz.

```yaml
variables:
  GIT_DEPTH: 0
```

---

### Jenkins (Declarative Pipeline)

```groovy
pipeline {
    agent any
    stages {
        stage('Versiyon') {
            steps {
                script {
                    def versionJson = sh(
                        script: '''docker run --rm \
                            -v $(pwd):/workspace -w /workspace \
                            ghcr.io/miraccan00/go-semver:latest''',
                        returnStdout: true
                    ).trim()
                    env.APP_VERSION = sh(
                        script: "echo '${versionJson}' | jq -r '.MajorMinorPatch'",
                        returnStdout: true
                    ).trim()
                    echo "Derlenen versiyon: ${env.APP_VERSION}"
                }
            }
        }
        stage('Build') {
            steps {
                sh "docker build -t myapp:${env.APP_VERSION} ."
            }
        }
    }
}
```

---

### CI için Kullanılabilir JSON Alanları

| Alan | Örnek | Yaygın Kullanım |
|---|---|---|
| `MajorMinorPatch` | `1.3.0` | Docker image tag, artifact adı |
| `FullSemVer` | `1.3.0+42` | Build metadata |
| `ShortSha` | `abc1234` | İzlenebilirlik tag'i |
| `BranchName` | `main` | Branch'e göre koşullu mantık |
| `EscapedBranchName` | `feature-login` | Image tag'lerinde güvenli kullanım |
| `CommitDate` | `2026-04-10` | Release notları |
| `UncommittedChanges` | `0` | Koruma: dirty repo ise build'i durdur |

Uncommitted değişiklik varsa pipeline'ı durdur:

```bash
DIRTY=$(echo "$VERSION_JSON" | jq '.UncommittedChanges')
if [ "$DIRTY" -gt 0 ]; then
  echo "HATA: $DIRTY commit edilmemiş değişiklik tespit edildi. Build öncesi commit yapın."
  exit 1
fi
```

---

## Bilinen Kısıtlamalar / Edge Case'ler

Aşağıdaki durumlar şu an **tam desteklenmiyor** — CI/CD entegrasyonunda dikkat edilmeli:

| Durum | Mevcut Davranış |
|---|---|
| Detached HEAD (CI checkout) | `BranchName` boş string döner, hata yok |
| Pre-release versiyonlar (`1.0.0-rc.1`) | `VersionInfo` alanları var ama doldurulmaz |
| Geçersiz VERSION içeriği (`abc`) | `strconv.Atoi` hatayı yutarak `0` döner |
| Merge commit mesajları | Conventional prefix yoksa bump olmaz |
| Çoklu bump kuralı (aynı commit) | İlk eşleşen kazanır: major → minor → patch |
| `git` binary bulunamadı | Git alanları sıfır döner, hata fırlatılmaz |
| Boş repo (hiç commit yok) | GitVersion.yml fallback çalışır; git alanları boş string |

## Test

```bash
go test ./internal/semver/...
```

Mevcut kapsam: dosya okuma/yazma round-trip, `ParseVersion`, `BumpByCommitMessage` (6 senaryo).

## Proje Yapısı

```
cmd/new-semver/main.go          # CLI giriş noktası
internal/semver/semver.go       # Core kütüphane
internal/semver/semver_test.go  # Unit testler
GitVersion.yml                  # Fallback konfigürasyon
VERSION                         # Aktif versiyon dosyası (gitignore'a eklemeyin)
```

## Katkıda Bulunma

Pull request'ler memnuniyetle karşılanır. Yeni özellik eklerken lütfen edge case'leri `internal/semver/semver_test.go` içinde test edin.
