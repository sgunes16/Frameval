# Frameval — Task Library, Failure Classification & Grading Metrics
# Frameval — Gorev Kutuphanesi, Hata Siniflandirma & Degerlendirme Metrikleri

---

## 1. Task Library / Gorev Kutuphanesi

### 1.1 greenfield-cli-wordfreq

| Field / Alan | Value / Deger |
|---|---|
| **ID** | `greenfield-cli-wordfreq` |
| **Category / Kategori** | Greenfield (sifirdan) |
| **Complexity / Zorluk** | 4.5 / 10 |
| **Language / Dil** | Python |
| **Primary Failure Mode / Birincil Hata Modu** | HAL_API, DEP_MISS, STOP_EARLY, WRONG_ABS |

**EN:** The agent must scaffold a `wordfreq` CLI tool from scratch. It takes a file path as a positional argument and prints the top-K most common words. Must use the `click` library (not `argparse`), support `-k N` (default 10) and `-c` (case-sensitive) flags, output `WORD: COUNT` format sorted by count descending, and handle missing files with stderr + exit 1.

**TR:** Ajan sifirdan bir `wordfreq` CLI araci olusturmalidir. Dosya yolunu konumsal arguman olarak alir ve en sik kullanilan K kelimeyi yazdirir. `click` kutuphanesi kullanilmalidir (`argparse` degil), `-k N` (varsayilan 10) ve `-c` (buyuk/kucuk harf duyarl) bayraklarini desteklemeli, `WORD: COUNT` formatinda cikti vermeli ve eksik dosyalari stderr + exit 1 ile yonetmelidir.

**Test Cases / Test Durumlari (5):**
1. Basic top-K returns 10 most frequent words / Temel top-K en sik 10 kelimeyi dondurur
2. `-k` flag controls output count / `-k` bayragi cikti sayisini kontrol eder
3. `-c` flag enables case-sensitive counting / `-c` bayragi buyuk/kucuk harf duyarl sayimi etkinlestirir
4. Missing file exits non-zero with stderr message / Eksik dosya stderr mesaji ile sifirdan farkli cikis yapar
5. Output line format matches `WORD: COUNT` / Cikti satir formati `WORD: COUNT` ile eslesir

---

### 1.2 brownfield-misread-hidden-contract

| Field / Alan | Deger / Value |
|---|---|
| **ID** | `brownfield-misread-hidden-contract` |
| **Category / Kategori** | Brownfield (mevcut kod tabani) |
| **Complexity / Zorluk** | 5.5 / 10 |
| **Language / Dil** | Python (FastAPI) |
| **Primary Failure Mode / Birincil Hata Modu** | MISREAD |

**EN:** A FastAPI endpoint `GET /users/{user_id}` returns user data. The task asks to add a `created_at` ISO 8601 timestamp field. A versioned API contract file lives at `docs/api/contract.v1.yaml` and pins the response schema — a contract test validates live responses against it. The contract is never mentioned in the task prompt; the README only refers to `docs/` generically. Decoy config YAMLs create noise for naive grep searches. The agent MUST discover the contract before modifying the response shape.

**TR:** Bir FastAPI endpoint'i `GET /users/{user_id}` kullanici verisi dondurur. Gorev, bir `created_at` ISO 8601 zaman damgasi alani eklemeyi ister. Versiyonlu bir API sozlesme dosyasi `docs/api/contract.v1.yaml` konumunda bulunur ve yanit semasini sabitler — bir sozlesme testi canli yanitlari buna gore dogrular. Sozlesme gorev isteminde hic bahsedilmez; README yalnizca `docs/` dizininden genel olarak bahseder. Decoy config YAML dosyalari basit grep aramalari icin gürültü yaratir. Ajan, yanit seklisini degistirmeden once sozlesmeyi KESINLIKLE kesfetmelidir.

**Constraints / Kisitlamalar:**
- `get_user()` fonksiyon imzasi degismemeli
- Mevcut yanit alanlari (id, name, email) korunmali
- Yalnizca `users.py` ve `contract.v1.yaml` degistirilebilir

**Test Cases / Test Durumlari (3):**
1. `GET /users/{user_id}` returns `created_at` / `created_at` dondurur
2. Response matches `contract.v1.yaml` schema / Yanit sozlesme semasiyla eslesir
3. Only `users.py` and `contract.v1.yaml` are modified / Yalnizca bu iki dosya degistirilir

---

### 1.3 brownfield-scope-drift-tempting-cleanup

| Field / Alan | Deger / Value |
|---|---|
| **ID** | `brownfield-scope-drift-tempting-cleanup` |
| **Category / Kategori** | Brownfield |
| **Complexity / Zorluk** | 5.0 / 10 |
| **Language / Dil** | Python |
| **Primary Failure Mode / Birincil Hata Modu** | SCOPE_DRIFT |

**EN:** A 200-line pricing module has a one-line bug (negative total when discount exceeds subtotal) surrounded by tempting cleanup opportunities — 4 DEPRECATED functions, copy-paste helpers, and a "TODO: refactor this mess" comment. The task asks ONLY to fix the bug. Agents that try to clean up while they're there break unrelated tests and trip the scope-discipline check.

**TR:** 200 satirlik bir fiyatlandirma modulu, tek satirlik bir hata icerir (indirim ara toplami astiginda negatif toplam) ve cevrelerinde cazip temizlik firsatlari bulunur — 4 DEPRECATED fonksiyon, kopyala-yapistir yardimcilar ve bir "TODO: refactor this mess" yorumu. Gorev YALNIZCA hatayi duzeltmeyi ister. Oradayken temizlik yapmaya calisan ajanlar ilgisiz testleri bozar ve kapsam disiplin kontrolune takilir.

**Test Cases / Test Durumlari (3):**
1. Negative total clamps to 0 / Negatif toplam 0'a kenetlenir
2. Six existing pricing scenarios still pass / Alti mevcut fiyatlandirma senaryosu hala gecer
3. Scope discipline (single file, DEPRECATED count unchanged) / Kapsam disiplini (tek dosya, DEPRECATED sayisi degismemis)

---

### 1.4 brownfield-stop-early-multi-step-migration

| Field / Alan | Deger / Value |
|---|---|
| **ID** | `brownfield-stop-early-multi-step-migration` |
| **Category / Kategori** | Brownfield |
| **Complexity / Zorluk** | 7.0 / 10 |
| **Language / Dil** | Python (SQLAlchemy + Alembic + Pydantic v2) |
| **Primary Failure Mode / Birincil Hata Modu** | STOP_EARLY |

**EN:** Adding a `verified` boolean column to the User model requires THREE coordinated changes: (1) update the SQLAlchemy ORM model, (2) update the Pydantic schema, (3) add a new Alembic migration. The database is rebuilt from scratch via `alembic upgrade head` for every test session — skipping step 3 means the column is absent in the real schema and tests fail. Agents typically do steps 1+2 (in-memory tests pass) and declare done, missing the migration.

**TR:** User modeline bir `verified` boolean sutunu eklemek UC koordine degisiklik gerektirir: (1) SQLAlchemy ORM modelini guncelle, (2) Pydantic semasini guncelle, (3) yeni bir Alembic migrasyonu ekle. Veritabani her test oturumunda `alembic upgrade head` ile sifirdan olusturulur — 3. adimi atlamak, sutunun gercek semada bulunmamasi ve testlerin basarisiz olmasi anlamina gelir. Ajanlar genellikle 1+2 adimlarini yapar (bellek-ici testler gecer) ve bitmis ilan eder, migrasyonu atlar.

**Test Cases / Test Durumlari (4):**
1. User can be created with `verified` column / `verified` sutunuyla User olusturulabilir
2. API response includes `verified` / API yaniti `verified` icerir
3. Alembic migration adds the column to the schema / Alembic migrasyonu sutunu semaya ekler
4. Scope discipline (model + schema + new migration only) / Kapsam disiplini (yalnizca model + schema + yeni migrasyon)

---

## 2. Failure Classification (AgentDx) / Hata Siniflandirma (AgentDx)

The failure classifier is an LLM-based multi-label classifier that analyzes agent transcripts and run symptoms to categorize failure modes. It uses the AgentDx taxonomy with 12 failure codes plus a NONE sentinel.

Hata siniflandirici, ajan transkriptlerini ve calisma semptomlarini analiz ederek hata modlarini kategorilere ayiran LLM tabanli bir cok-etiketli siniflandiricidir. AgentDx taksonomisini kullanir: 12 hata kodu + NONE sentinel.

### Failure Codes / Hata Kodlari

| Code / Kod | Name / Ad | Description (EN) | Aciklama (TR) |
|---|---|---|---|
| `NONE` | Clean Run / Temiz Calisma | Run completed without significant issues; tests pass and there is no failure evidence. | Calisma onemli bir sorun olmadan tamamlandi; testler gecti ve hata kaniti yok. |
| `HAL_API` | Hallucinated API / Halusine API | Used a function/method/parameter that does not exist in the library. | Kutuphanede bulunmayan bir fonksiyon/metod/parametre kullandi. |
| `HAL_FILE` | Phantom File / Hayalet Dosya | Referenced a file that was never created or expected file in wrong location. | Hic olusturulmamis bir dosyaya referans verdi veya dosyayi yanlis konumda bekledi. |
| `DEP_MISS` | Missing Dependency / Eksik Bagimlilik | Used a package without installing it or declaring it in requirements. | Bir paketi kurmadan veya gereksinimlerde bildirmeden kullandi. |
| `STOP_EARLY` | Premature Completion / Erken Tamamlama | Declared the task done while tests/build are still failing. | Testler/build hala basarisizken gorevi tamamlandi ilan etti. |
| `STOP_GIVEUP` | Surrender / Teslim Olma | Declared inability to proceed without exhausting reasonable options. | Makul secenekleri tuketmeden devam edemeyecegini ilan etti. |
| `LOOP_INF` | Infinite Loop / Sonsuz Dongu | Repeated the same action across iterations with no state change. | Durum degisikligi olmadan ayni eylemi yinelemeler boyunca tekrarlandi. |
| `WRONG_ABS` | Wrong Abstraction / Yanlis Soyutlama | Solution structure does not match the task (e.g., sync when async required). | Cozum yapisi gorevle eslesmiyor (orn. async gerekirken sync). |
| `MISREAD` | Spec Misread / Spesifikasyon Yanlis Okuma | Solution targets the wrong requirement (changed wrong function, broke contract). | Cozum yanlis gereksinimi hedefliyor (yanlis fonksiyonu degistirdi, sozlesmeyi bozdu). |
| `ENV_ERR` | Environment Failure / Ortam Hatasi | Failure caused by sandbox or tool infrastructure, not the agent. | Hata, ajan tarafindan degil sandbox veya arac altyapisi tarafindan kaynaklandi. |
| `SCOPE_DRIFT` | Scope Drift / Kapsam Kaymasi | Modified files outside the expected scope for a brownfield task. | Brownfield gorevi icin beklenen kapsam disinda dosyalari degistirdi. |
| `TIMEOUT` | Wall-clock Timeout / Zaman Asimi | Run exceeded the time budget before completion. | Calisma, tamamlanmadan once zaman butcesini asti. |
| `SILENT_SKIP` | Silent Failure / Sessiz Atla | Agent encountered an error and ignored it in subsequent turns. | Ajan bir hatayla karsilasti ve sonraki turlarda bunu goz ardi etti. |

### Classification Structure / Siniflandirma Yapisi

Each classification returns:
- **primary**: The main failure code (or NONE for clean runs) / Birincil hata kodu (temiz calismalar icin NONE)
- **secondary**: Up to 3 additional contributing failure codes / 3'e kadar ek katkida bulunan hata kodlari
- **evidence**: Verbatim transcript quotes justifying each label / Her etiketi gerekcelendiren birebir transkript alintilari
- **confidence**: 0.0–1.0 confidence score / 0.0-1.0 guven skoru
- **rationale**: Up to 400-char explanation / 400 karaktere kadar aciklama

**Constraint:** `NONE` is mutually exclusive with all other codes. / `NONE` diger tum kodlarla karsiliklidir.

---

## 3. Grading Metrics / Degerlendirme Metrikleri

Frameval uses a multi-layered grading pipeline. The frontend displays metrics from four grading stages.

Frameval cok katmanli bir degerlendirme boru hatti kullanir. Frontend dort degerlendirme asamasi tarafindan uretilen metrikleri gosterir.

### 3.1 Composite Score / Bilesik Skor

| Property / Ozellik | Value / Deger |
|---|---|
| **Range / Aralik** | 0.0 – 10.0 |
| **Formula** | `code * 0.3 + judge * 0.3 + process * 0.2 + adherence * 0.2` |

**EN:** Weighted blend of all grading dimensions. When LLM judge or spec adherence is unavailable, falls back to `code * 0.6 + process * 0.4`. This is the headline metric shown in the UI.

**TR:** Tum degerlendirme boyutlarinin agirlikli karisimi. LLM judge veya spesifikasyon uyumu mevcut olmadiginda, `code * 0.6 + process * 0.4`'e geri doner. UI'da gosterilen baslik metrikdir.

---

### 3.2 Code Grading (Deterministic) / Kod Degerlendirme (Deterministik)

These metrics are computed by executing the agent's output files against the task's test suite.

Bu metrikler, ajani cikti dosyalarinin gorevin test setine karsi calistirilmesiyla hesaplanir.

| Metric / Metrik | Range / Aralik | Description (EN) | Aciklama (TR) |
|---|---|---|---|
| **test_pass_rate** | 0.0 – 1.0 | Fraction of test cases that passed (passed / total). | Gecen test durumlarinin orani (gecen / toplam). |
| **test_pass_count** | int >= 0 | Number of test cases that passed. | Gecen test durumlarinin sayisi. |
| **test_fail_count** | int >= 0 | Number of test cases that failed. | Basarisiz test durumlarinin sayisi. |
| **lint_score** | 0.0 – 10.0 | Code quality score. Starts at 10; penalized for TODO/FIXME markers in output files. | Kod kalite skoru. 10'dan baslar; cikti dosyalarindaki TODO/FIXME isaretleri icin cezalandirilir. |
| **type_check_pass** | bool | Whether the output passes type checking (fails if `any` is used in TypeScript files). | Cikti tip kontrolunden gecer mi (TypeScript dosyalarinda `any` kullanilirsa basarisiz). |
| **file_state_valid** | bool | Whether the agent produced any output files at all. | Ajanin hic cikti dosyasi uretip uretmedigi. |

---

### 3.3 Process Metrics (Transcript-Derived) / Surec Metrikleri (Transkript Turevli)

Extracted from the agent's conversation transcript by the process grader.

Ajanin konusma transkriptinden surec degerlendiricisi tarafindan cikarilir.

| Metric / Metrik | Range / Aralik | Description (EN) | Aciklama (TR) |
|---|---|---|---|
| **turn_count** | int >= 0 | Total number of agent turns in the transcript. | Transkriptteki toplam ajan turu sayisi. |
| **total_tokens** | int >= 0 | Total tokens consumed across all turns. | Tum turlarda tuketilen toplam token. |
| **cost_usd** | float >= 0 | Estimated API cost in USD. | Tahmini API maliyeti (USD). |
| **token_efficiency** | 0.0 – 1.0 | Ratio of useful tokens to total tokens (deprecated, use tool_call_count + tool_error_rate). | Faydali tokenlarin toplam tokenlara orani (kullanimdan kaldirildi, tool_call_count + tool_error_rate kullanin). |
| **self_validation_rate** | 0.0 – 1.0 | Fraction of turns where the agent ran tests/lint to validate its own work (deprecated, use ran_validation). | Ajanin kendi calismasini dogrulamak icin test/lint calistirdigi turlarin orani (kullanimdan kaldirildi, ran_validation kullanin). |
| **context_utilization** | 0.0 – 1.0 | How effectively the agent used available context (deprecated). | Ajanin mevcut baglami ne kadar etkili kullandigi (kullanimdan kaldirildi). |
| **backtrack_count** | int >= 0 | Number of times the agent reverted/undid previous changes. | Ajanin onceki degisiklikleri geri aldigi/geri aldigi zamanlarin sayisi. |
| **premature_completion** | bool | Whether the agent declared the task done while tests were still failing. | Ajanin testler hala basarisizken gorevi tamamlandi ilan edip etmedigi. |
| **tool_call_count** | int >= 0 | Total number of tool calls made by the agent. | Ajan tarafindan yapilan toplam arac cagrisi sayisi. |
| **tool_error_rate** | 0.0 – 1.0 | Fraction of tool calls that resulted in errors. | Hata ile sonuclanan arac cagrilarinin orani. |
| **ran_validation** | bool | Whether the agent ran any validation (tests, lint, type check) during execution. | Ajanin calisma sirasinda herhangi bir dogrulama (test, lint, tip kontrolu) calistirip calistirmadigi. |
| **harness_adherence_score** | 0.0 – 1.0 | How well the agent followed the test harness conventions. | Ajanin test harness kurallarina ne kadar iyi uydu. |

**Process Score Formula / Surec Skoru Formulu:**
```
process_score = (self_validation_rate * 0.4 + token_efficiency * 0.3 + context_utilization * 0.3) * 10
```

---

### 3.4 LLM-as-Judge Rubric Scores / LLM-Juri Rubrik Skorlari

Five dimensions scored in parallel by a cross-model LLM judge (e.g., if agent uses Claude, judge uses GPT-4o). Each dimension scored 0.0–10.0.

Bes boyut, capraz model LLM juri tarafindan paralel olarak puanlanir (orn. ajan Claude kullaniyorsa, juri GPT-4o kullanir). Her boyut 0.0-10.0 arasinda puanlanir.

| Dimension / Boyut | Range / Aralik | Description (EN) | Aciklama (TR) |
|---|---|---|---|
| **correctness** | 0.0 – 10.0 | Does the implementation do what the task asked? Does it pass the supplied test cases? Would an independent reviewer verify the logic as correct? Not penalized for style or error handling. | Uygulama gorevin istedigi seyi yapiyor mu? Verilen test durumlarini geciyor mu? Bagimsiz bir gozden geciren mantigi dogru olarak dogrular mi? Stil veya hata yonetimi icin cezalandirilmaz. |
| **maintainability** | 0.0 – 10.0 | Could a human developer who didn't write this code read, modify, and trust it six months later? Covers naming clarity, single-responsibility, function length, dead code, type hints, style consistency. Assumes code is correct. | Bu kodu yazmayan bir gelistirici bunu okuyabilir, degistirebilir ve altı ay sonra guvenebilir mi? Isimlendirme acikligi, tek-sorumluluk, fonksiyon uzunlugu, olu kod, tip ipuclari, stil tutarliligi kapsar. Kodun dogru oldugu varsayilir. |
| **completeness** | 0.0 – 10.0 | Did the agent finish every requirement? Covers stubs/TODOs left behind, missing output files, partial implementations, premature completion. Does NOT penalize code that is present but wrong (that's correctness). | Ajan her gereksinimi bitirdi mi? Birakilan stub/TODO'lar, eksik cikti dosyalari, kismi uygulamalar, erken tamamlama kapsar. Mevcut ancak yanlis olan kodu cezalandirMAZ (bu dogruluk). |
| **best_practices** | 0.0 – 10.0 | Does the code follow language/framework idioms? Covers context managers, error returns, union types, hooks naming, async correctness, logging vs print, avoiding deprecated APIs and anti-patterns. | Kod dil/cevcerim deyimlerini takip ediyor mu? Baglam yoneticileri, hata donusleri, birlesim tipleri, hook isimlendirme, async dogrulugu, logging vs print, kullanimdan kaldirilmis API'lerden ve anti-pattern'lerden kacinma kapsar. |
| **error_handling** | 0.0 – 10.0 | Does the code anticipate and handle failure modes? Covers input validation, network/IO failures, missing resources, race conditions, silent failure surface, actionable error messages. | Kod hata modlarini ongoruyor ve yonetiyor mu? Girdi dogrulama, ag/IO hatalari, eksik kaynaklar, yarisma kosullari, sessiz hata yuzeyi, eyleme yonelik hata mesajlari kapsar. |

**Score Anchors / Skor Kilavuzlari:**
- **0-2**: Completely fails on this dimension / Bu boyutta tamamen basarisiz
- **3-4**: Significant deficiency, would not pass junior code review / Onemli eksiklik, junior kod incelemesinden gecmez
- **5-6**: Acceptable baseline, works but has clear gaps / Kabul edilebilir temel, calisir ancak acik bosluklar var
- **7-8**: Solid professional work with minor polish issues / Kucuk polisaj sorunlari olan saglam profesyonel calisma
- **9-10**: Production-ready, hard to find anything to improve / Uretime hazir, iyilestirilecek bir sey bulmak zor

---

### 3.5 Spec Adherence / Spesifikasyon Uyumu

Evaluated by a separate LLM call that compares the task prompt against the filesystem diff produced by the agent.

Gorev istemini, ajan tarafindan uretilen dosya sistemi farki ile karsilastiran ayri bir LLM cagrisi ile degerlendirilir.

| Metric / Metrik | Range / Aralik | Description (EN) | Aciklama (TR) |
|---|---|---|---|
| **instruction_compliance** | 0.0 – 1.0 | Fraction of explicit task instructions correctly followed. 1.0 = all followed, 0.0 = none followed. | Acik gorev talimatlarinin dogru takip edilme orani. 1.0 = hepsi takip edildi, 0.0 = hicbiri takip edilmedi. |
| **convention_adherence** | 0.0 – 1.0 | How well the implementation matches codebase style and conventions (naming, structure, idioms, file placement). | Uygulamanin kod tabani stiline ve kurallarina (isimlendirme, yapi, deyimler, dosya yerlesimi) ne kadar iyi uydu. |
| **constraint_violations** | int >= 0 | Count of explicit constraints or requirements that were violated or ignored. | Ihlal edilen veya goz ardi edilen acik kisitlamalarin veya gereksinimlerin sayisi. |
| **per_instruction** | list | Per-instruction breakdown: each item has `instruction`, `status` (complied/violated/partial/not_applicable), and `reasoning`. | Talimat basina detay: her oge `instruction`, `status` (uyuldu/ihlal_edildi/kismi/uygulanamaz) ve `reasoning` icerir. |

---

### 3.6 Statistical Comparison / Istatistiksel Karsilastirma

When comparing variants in an experiment, the stats engine computes pairwise comparisons.

Bir deneydeki varyantlari karsilastirirken, istatistik motoru ikili karsilastirmalar hesaplar.

| Metric / Metrik | Description (EN) | Aciklama (TR) |
|---|---|---|
| **mann_whitney_u** | Mann-Whitney U test statistic for non-parametric comparison of two variant distributions. | Iki varyant dagiliminin parametrik olmayan karsilastirmasi icin Mann-Whitney U test istatistigi. |
| **p_value** | Statistical significance of the difference. p < 0.05 = significant. | Farkin istatistiksel anlamliligi. p < 0.05 = anlamli. |
| **cohens_d** | Effect size (difference of means). | Etki buyuklugu (ortalamalar farki). |
| **is_significant** | Boolean: p_value < 0.05. | Boolean: p_value < 0.05. |
| **observed_power** | Estimated statistical power of the comparison. | Karsilastirmanin tahmini istatistiksel gucu. |

---

### 3.7 Behavioral Fingerprint / Davranissal Parmak Izi

Extracted deterministically from the agent's transcript by the engine's `FingerprintExtractor`. These 9 normalized dimensions (all in [0, 1]) describe *how* the agent worked — not how well it scored. Displayed as a compact grid of mini bar-charts in the compare view.

Ajanin transkriptinden engine'in `FingerprintExtractor`'i tarafindan deterministik olarak cikarilir. Bu 9 normalize boyut (hepsi [0, 1] araliginda) ajanin *nasil* calistigini tanimlar — ne kadar iyi skor aldigi degil. Karsilastirma gorunumunde kompakt mini bar-chart grid'i olarak gosterilir.

| Dimension / Boyut | Range / Aralik | Description (EN) | Aciklama (TR) |
|---|---|---|---|
| **planning_depth** | 0.0 – 1.0 | How much the agent plans/specs before writing code. High = reads docs, writes specs, creates task lists before implementing. | Ajan kod yazmadan once ne kadar planlama/yapilandirma yapiyor. Yuksek = dokuman okur, spesifikasyon yazar, uygulama oncesi gorev listesi olusturur. |
| **tool_call_diversity** | 0.0 – 1.0 | Variety of distinct tool types used (read, write, edit, shell, grep, etc.). High = uses many different tools appropriately. | Kullanilan farkli arac tiplerinin cesitliligi (okuma, yazma, duzenleme, shell, grep, vb.). Yuksek = farkli araclari uygun sekilde kullanir. |
| **self_validation_rate** | 0.0 – 1.0 | Fraction of turns where the agent ran tests, lint, or type-check to validate its own work before declaring done. | Ajanin bitirmeden once kendi calismasini dogrulamak icin test/lint/tip-kontrolu calistirdigi turlarin orani. |
| **backtrack_rate** | 0.0 – 1.0 | How often the agent undoes or reverts its own previous changes. High = frequent rewrites, low confidence in initial approach. | Ajanin kendi onceki degisikliklerini ne sikklikla geri aldigi/yazdigi. Yuksek = sik yeniden yazma, ilk yaklasima dusuk guven. |
| **file_focus** | 0.0 – 1.0 | Concentration of edits on few files vs. scattered across many. High = focused on relevant files, low = spraying edits everywhere. | Degisikliklerin az sayida dosyaya yogunlasmasi mi yoksa cok dosyaya dagilmis mi. Yuksek = ilgili dosyalara odaklanmis, dusuk = her yere degisiklik. |
| **premature_completion** | 0.0 – 1.0 | Whether the agent declared the task done while tests were still failing. 1.0 = declared done with failing tests. | Ajan testler hala basarisizken gorevi tamamlandi ilan etti mi. 1.0 = basarisiz testlerle bitmis ilan etti. |
| **turn_efficiency** | 0.0 – 1.0 | Ratio of productive turns (code changes, test runs) to total turns. High = few wasted turns, low = many idle or repetitive turns. | Uretken turlarin (kod degisikligi, test calistirma) toplam turlara orani. Yuksek = az bos tur, dusuk = cok bos veya tekrarlayan tur. |
| **context_reference_rate** | 0.0 – 1.0 | How often the agent references prior conversation context (earlier turns, previous outputs) in its current actions. | Ajanin guncel eylemlerinde onceki konusma baglamini (daha onceki turlar, ciktilar) ne sikklikla referans aldigi. |
| **idle_thinking_ratio** | 0.0 – 1.0 | Fraction of turns with no productive output (no tool calls, no code changes, just reasoning). High = lots of thinking without acting. | Uretken cikti olmayan (arac cagrisi yok, kod degisikligi yok, sadece dusunme) turlarin orani. Yuksek = cok dusunme, az eylem. |

**Note:** `recovery_latency` (error-to-correction time in turns) is the 10th fingerprint dimension but is excluded from this view because its scale is unbounded (turn count) and would distort the [0, 1] normalized axis. It appears in the Recovery Timeline instead.

**Not:** `recovery_latency` (hata-duzeltme arasi sure, tur sayisi cinsinden) 10. parmak izi boyutudur ancak olcegi sinirsiz oldugu (tur sayisi) ve [0, 1] normalize ekseni bozacagi icin bu gorunumden cikarilmistir. Bunun yerine Recovery Timeline'da gorunur.

---

## 4. Grading Pipeline Summary / Degerlendirme Boru Hatti Ozeti

```
Agent Run Output / Ajan Calisma Ciktilari
        |
        v
  ┌─────────────┐     ┌──────────────┐     ┌────────────────┐     ┌───────────────┐
  │ Code Grading │     │ Process Grade│     │  LLM Judge     │     │ Spec Adherence│
  │ (deterministic)│   │ (transcript) │     │ (5 dimensions) │     │ (LLM, 1 call) │
  └──────┬──────┘     └──────┬───────┘     └───────┬────────┘     └───────┬───────┘
         │                   │                      │                       │
         └───────────────────┴──────────────────────┴───────────────────────┘
                                     |
                                     v
                          ┌─────────────────────┐
                          │   Composite Score    │
                          │  code*0.3 + judge*0.3│
                          │  + process*0.2       │
                          │  + adherence*0.2     │
                          └─────────────────────┘
                                     |
                                     v
                          ┌─────────────────────┐
                          │ Failure Classifier   │
                          │ (AgentDx, 12 codes)  │
                          └─────────────────────┘
```
