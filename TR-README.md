# go-semver

Git commit mesajlarına dayalı otomatik semantic versioning aracı. [Conventional Commits](https://www.conventionalcommits.org/) kurallarını okuyarak `VERSION` dosyasını günceller ve CI/CD pipeline'larında kullanılmak üzere zengin JSON metadata çıktısı üretir.

## Kurulum & Derleme

```bash
go build -o new-semver ./cmd/new-semver
```

## Kullanım

Repo kökünde çalıştırın:

```bash
./new-semver
```

Stdout'a JSON metadata basar, VERSION dosyasını günceller.

## Versiyon Belirleme Mantığı

```
VERSION dosyası var?
  ├── Evet → dosyadaki versiyonu oku
  └── Hayır → GitVersion.yml içindeki next-version: alanını kullan

Son commit mesajı var mı?
  ├── Evet → BumpByCommitMessage() ile versiyon artır, VERSION dosyasına yaz
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

## Çıktı Formatı

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
| `VERSION` | `X.Y.Z` formatında aktif versiyon (araç tarafından yönetilir) |
| `GitVersion.yml` | `next-version:` ile fallback başlangıç versiyonu |

## GitVersion.yml

```yaml
next-version: 0.0.1       # VERSION dosyası yoksa kullanılır
mode: ContinuousDeployment
```

## Bilinen Kısıtlamalar / Mevcut Edge Case'ler

Aşağıdaki durumlar şu an **tam desteklenmiyor** — CI/CD entegrasyonunda dikkat edilmeli:

| Durum | Mevcut Davranış |
|---|---|
| Detached HEAD (CI checkout) | `BranchName` boş string döner, hata yok |
| Pre-release versiyonlar (`1.0.0-rc.1`) | `VersionInfo` alanları var ama doldurulmaz |
| Geçersiz VERSION içeriği (`abc`) | `strconv.Atoi` hatayı yutarak `0` döner |
| Merge commit mesajları | Conventional prefix yoksa bump olmaz |
| Çoklu bump (aynı commit'te birden fazla kural) | İlk eşleşen kural kazanır (major > minor > patch) |
| Git binary bulunamadı | Sıfır değerler döner, hata fırlatılmaz |
| Boş repo (hiç commit yok) | GitVersion.yml fallback'i çalışır, JSON'daki git alanları boş |

## Test

```bash
go test ./internal/semver/...
```

Mevcut testler: versiyon dosyası okuma/yazma, `ParseVersion`, `BumpByCommitMessage` (6 senaryo).

## Mimari

```
cmd/new-semver/main.go          # CLI giriş noktası
internal/semver/semver.go       # Tüm core logic
internal/semver/semver_test.go  # Unit testler
GitVersion.yml                  # Fallback konfigürasyon
VERSION                         # Aktif versiyon dosyası (gitignore'a eklemeyin)
```
