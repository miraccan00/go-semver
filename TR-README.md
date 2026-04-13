# go-semver

[Conventional Commits](https://www.conventionalcommits.org/) kurallarını okuyarak Git repolarında semantic versioning'i otomatikleştiren, CI/CD pipeline'ları için zengin JSON metadata üreten hafif bir CLI aracı.

Versiyon artışları **her zaman commit mesajlarına göre belirlenir** — branch ismine göre değil. Branch yalnızca otomatik tag oluşturmanın nerede tetikleneceğini (yalnızca mainline) kontrol eder.

## Kurulum & Derleme

```bash
go build -o new-semver ./cmd/new-semver
```

## Kullanım

Repo kökünde çalıştırın:

```bash
./new-semver
```

Stdout'a JSON metadata basar. Hiçbir dosya yazılmaz veya değiştirilmez (mainline merge tespit edildiğinde yerel git reposuna versiyon tag'i oluşturulur).

## Versiyon Belirleme Mantığı

```
Semver git etiketi var mı? (örn. v1.2.3)
  ├── Hayır → GitVersion.yml içindeki next-version: değerini olduğu gibi yaz
  └── Evet  → etiketten bu yana olan commit'leri topla
                └── Yeni commit var mı?
                      ├── Hayır → etiket versiyonunu değiştirmeden yaz
                      └── Evet  → mainline + merge tespit edildi mi?
                                    ├── Evet → commit'lere göre bump + git tag oluştur
                                    └── Hayır → commit'lere göre bump (tag yok)
```

### Conventional Commits → Versiyon Artışı

Scope desteği mevcuttur (`feat(auth):`, `fix(parser):`). Tüm commit'ler arasında en yüksek bump kazanır.

| Commit Prefix | Örnek | Etki |
|---|---|---|
| `fix:` / `fix(scope):` | `fix: null pointer in auth` | PATCH artışı |
| `feat:` / `feat(scope):` | `feat(api): add retry logic` | MINOR artışı |
| `feat!:` / `fix!:` (opsiyonel scope ile) | `feat!: drop v1 API` | MAJOR artışı |
| Footer'da `BREAKING CHANGE:` | `BREAKING CHANGE: schema renamed` | MAJOR artışı |
| Footer'da `BREAKING-CHANGE:` | `BREAKING-CHANGE: env vars changed` | MAJOR artışı |
| Diğerleri (`chore:`, `docs:`, vb.) | `chore: update deps` | Artış yok |

> Eşleştirme büyük/küçük harf duyarsızdır. Birden fazla commit varsa en yüksek bump kazanır.

## Branch Stratejisi

go-semver, GitFlow'a dayalı bir model izler. **Bump seviyesi her zaman commit'e göre belirlenir** — branch ismi yalnızca versiyon tag'inin oluşturulup oluşturulmayacağını kontrol eder.

```
Branch        Nereye merge edilir     Bump          Otomatik tag
────────────────────────────────────────────────────────────────
main          —                       —             Evet (merge'de)
develop       main                    commit-driven main'e merge'de
release/*     main + develop          commit-driven main'e merge'de
hotfix/*      main + develop          commit-driven main'e merge'de
feature/*     develop                 yok           hayır
```

- **develop** — entegrasyon branch'i. Feature branch'leri buraya merge edilir; develop main'e merge edilir.
- **release/\*** — bir release'i stabilize etmek için develop'tan ayrılır. Main'e merge edilir (tag tetikler) ve senkronize kalmak için develop'a geri merge edilir.
- **hotfix/\*** — acil düzeltme için main'den ayrılır. Main'e merge edilir (tag tetikler) ve develop'a geri merge edilir.
- **feature/\*** — kısa ömürlü geliştirme branch'leri. Tag yok, bump yok; JSON çıktısı yalnızca bilgilendirme amaçlıdır.

Araç mainline üzerinde çalışırken tanınan bir kaynak branch'in merge edildiğini tespit ettiğinde (SOURCE_BRANCH env var **veya** "Merge branch '…'" commit mesajı pattern'i aracılığıyla) versiyon tag'i otomatik olarak oluşturulur.

## Ortam Değişkenleri

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `MAINLINE_BRANCH` | `main` | Production branch'inin adı |
| `DEVELOP_BRANCH` | `develop` | Entegrasyon branch'inin adı (ekibiniz "test" olarak adlandırıyorsa onu kullanın) |
| `SOURCE_BRANCH` | _(boş)_ | Squash merge'lerde CI tarafından kaynak branch'i belirtmek için kullanılır; ayarlanmadığında commit mesajı tespitine geri döner |

### Squash Merge Desteği

Squash merge kullanıldığında main üzerindeki sonuç commit'te "Merge branch '…'" mesajı bulunmaz. Merge edilen branch'in adını (örn. `develop`, `release/1.2`, `hotfix/fix-x`) CI pipeline'ınızda `SOURCE_BRANCH` olarak ayarlayın; böylece go-semver merge'i tespit edip versiyon tag'ini oluşturabilir.

## Yapılandırma

### GitVersion.yml

İlk git etiketi oluşturulmadan önce versiyon kaynağı olarak kullanılır:

```yaml
next-version: 0.1.0       # henüz semver etiket yokken kullanılır
mode: ContinuousDeployment
```

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

> Container içinde `git` kurulu gelir. Aracın `.git/` ve `GitVersion.yml` dosyalarını okuyabilmesi için repo kökünüzü `/workspace` olarak mount edin.

---

## CI/CD Entegrasyonu

### Hızlı Başlangıç (tüm CI sistemleri için)

Git geçmişinin tam alındığından (`--depth 0`) emin olun ve versiyonu yakalamak için go-semver'ı çalıştırın:

```bash
VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
  ghcr.io/miraccan00/go-semver:latest)

APP_VERSION=$(echo "$VERSION_JSON" | jq -r '.MajorMinorPatch')
echo "Derlenen versiyon: $APP_VERSION"
```

Versiyon dosyası yazılmaz veya geri commit edilmez — go-semver versiyonlamayı tamamen git tag'leri üzerinden yönetir.

---

### GitHub Actions

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: write   # versiyon tag'ini push etmek için gerekli
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0          # zorunlu — go-semver tam git log'u okur

      - name: go-semver çalıştır
        id: semver
        env:
          SOURCE_BRANCH: ${{ github.head_ref }}   # squash merge tespiti için
        run: |
          VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
            -e SOURCE_BRANCH \
            ghcr.io/miraccan00/go-semver:latest)
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
  GIT_DEPTH: 0              # zorunlu — tam git geçmişini sağlar

semver:
  image: ghcr.io/miraccan00/go-semver:latest
  stage: .pre
  variables:
    SOURCE_BRANCH: $CI_MERGE_REQUEST_SOURCE_BRANCH_NAME   # squash merge tespiti için
  script:
    - VERSION_JSON=$(new-semver)
    - echo "APP_VERSION=$(echo $VERSION_JSON | jq -r '.MajorMinorPatch')" >> build.env
    - echo "SHORT_SHA=$(echo $VERSION_JSON | jq -r '.ShortSha')" >> build.env
  artifacts:
    reports:
      dotenv: build.env     # APP_VERSION'ı sonraki job'lara aktarır

build:
  stage: build
  needs: [semver]
  script:
    - docker build -t myapp:$APP_VERSION .
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

| Durum | Mevcut Davranış |
|---|---|
| Detached HEAD (CI checkout) | `BranchName` boş string döner, hata yok |
| SOURCE_BRANCH olmadan squash merge | Commit mesajı tespitine geri döner; tag oluşturulmasını garantilemek için `SOURCE_BRANCH` ayarlayın |
| Pre-release versiyonlar (`1.0.0-rc.1`) | `VersionInfo` alanları var ama doldurulmaz |
| Çoklu bump kuralı (birden fazla commit) | En yüksek bump kazanır: major → minor → patch |
| `git` binary bulunamadı | Git alanları sıfır döner, hata fırlatılmaz |
| Boş repo (hiç commit yok) | GitVersion.yml fallback çalışır; git alanları boş string |

## Test

```bash
go test ./internal/semver/...
```

Testler şunları kapsar: `ParseVersion`, `BumpByCommits` (scope ve `BREAKING-CHANGE` footer dahil Conventional Commits), `IsMainlineMerge`, `DetectMergeFrom*`, `GetMainlineBranch`, `GetDevelopBranch`, `GetSourceBranch`, `CreateVersionTag` ve tam uçtan uca branch akışı senaryoları.

## Proje Yapısı

```
cmd/new-semver/main.go                       # CLI giriş noktası
internal/semver/semver.go                    # Core kütüphane
internal/semver/semver_test.go               # Unit testler
internal/semver/semver_scenario_test.go      # Entegrasyon / senaryo testleri
GitVersion.yml                               # Fallback versiyon + branch konfigürasyon dokümantasyonu
ci-integration-example/                      # Kullanıma hazır CI pipeline şablonları
  github-actions.yml
  gitlab-ci.yml
  azure-pipelines.yml
```

## Katkıda Bulunma

Pull request'ler memnuniyetle karşılanır. Yeni özellik eklerken lütfen edge case'leri `internal/semver/semver_test.go` içinde, branch akışı davranışları için ise `semver_scenario_test.go` içinde test edin.
