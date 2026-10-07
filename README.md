# phonenumber

![Coverage](https://img.shields.io/badge/Coverage-99.4%25-brightgreen)
[![Run Tests](https://github.com/apifonica/phonenumber/actions/workflows/run-tests.yml/badge.svg)](https://github.com/apifonica/phonenumber/actions?query=branch%3Amaster)
## Description

Up-to-date list of mobile phone number formats - https://www.itu.int/oth/T0202.aspx?lang=en&parent=T0202

### Mobile vs landline sources

`MobileBeginWith` holds the national-number prefixes of mobile ranges; numbers of a valid length that match none of them are treated as landlines. Where a country gives mobile its own prefixes, its numbering plan is enough:

- [ITU-T national numbering plans](https://www.itu.int/oth/T0202.aspx?lang=en&parent=T0202). Link a country's page (e.g. [Mexico](https://www.itu.int/oth/T020200008A/en)), not the PDF: PDFs are replaced by new versions and old URLs return 404. The national regulator's own plan is usually newer.
- Google libphonenumber's [PhoneNumberMetadata.xml](https://github.com/google/libphonenumber/blob/master/resources/PhoneNumberMetadata.xml) has fixed-line and mobile patterns per region, but returns `FIXED_LINE_OR_MOBILE` where they overlap, e.g. all +1 countries and Mexico ([FAQ](https://github.com/google/libphonenumber/blob/master/FAQ.md#why-is-this-number-from-argentina-ar-or-mexico-mx-not-identified-as-the-right-number-type)).

In some countries mobile and fixed numbers share area codes and differ only per assigned number block. Prefixes from the plan cannot separate them; the regulator's block assignment file can. Before relying on it, classify a sample of delivered and undeliverable numbers from real traffic against the file.

| Country | Block assignment data | Line type field | Fixed↔mobile porting |
|---|---|---|---|
| Mexico +52 (generated, see below) | IFT [Plan Nacional de Numeración](https://sns.ift.org.mx/sns-frontend/planes-numeracion/descarga-publica.xhtml), CSV republished daily | `MODALIDAD`: `CPP`/`MPP` mobile, `FIJO` fixed | Not allowed |
| Argentina +54 | ENACOM [Asignaciones a la fecha](https://www.enacom.gob.ar/asignaciones-a-la-fecha_p445), XLS; prefix = `INDICATIVO` + `BLOQUE` | `SERVICIO`: `SRMC`/`STM`/`PCS` mobile, `SBT` fixed; `MODALIDAD` | Not allowed ([Res. 203/2018](https://www.enacom.gob.ar/multimedia/normativas/2018/res203MM.pdf), art. 5) |
| Dominican Republic +1 809/829/849 | INDOTEL [Códigos NXX](https://indotel.gob.do/telefonicas/), XLS per NPA-NXX | `Tipo de Asginación`: `LINEA MOVIL`, `LINEA FIJA`, `COMPARTIDO` (shared code, type unknown) | Rules cover only fixed→fixed and mobile→mobile |
| US, Canada +1 | NANPA [CO codes](https://www.nanpa.com/reports/co-code-reports/cocodes_assign) and [thousands blocks](https://www.nanpa.com/reports/thousands-block-reports/region); CNAC [CO code status](https://www.cnac.ca/co_codes/co_code_status.htm) | None: only the holder's OCN. OCN → carrier category is in the paid LERG | Allowed |

Block data shows the original allocation, not ported numbers. It is reliable only where fixed↔mobile porting is not allowed. In the US and Canada a landline number can be ported to a mobile carrier, so only a number portability or HLR lookup gives the current line type. This library's +1 entries list every area code as mobile, so they do not filter landlines.

Mexico's mobile prefixes are generated from the IFT file into `iso3166_mx.go`. Refresh them with `go generate ./...`.

golang port of AfterShip/phone

phonenumber is used to normalize the mobile phone or landline number into a E.164 format.

The common problem is user normally input the phone number in this way:

```
09061354467 or
090-6135-4467 or
(090) 6135-4467 or
819061354467 or
+819061354467 or
81(90) 6135-4467 or
+81(90) 6135-4467 or ...
```

What we want is always:

```
819061354467
```

## Installation
Run the following command to install the package:
```
go get github.com/apifonica/phonenumber
```

## Usage

### Clearing format
In this case landline numbers will be an invalid result:
```go
import "github.com/apifonica/phonenumber"

number := phonenumber.Parse("090-6135-4467", "JP")
fmt.Println(number)
// Output: 819061354467
```

In this case you can format numbers included landline numbers:
```go
import "github.com/apifonica/phonenumber"

number := phonenumber.ParseWithLandLine("+371 65 552-336", "LV")
fmt.Println(number)
// Output: 37165552336
```

### Get country for number
```go
import "github.com/apifonica/phonenumber"

// Get country with mobile and landline numbers
// Let's try to get country for Latvian landline number
includeLandLine := true
country := phonenumber.GetISO3166ByNumber("37165552336", includeLandLine)
fmt.Println(country.CountryName)
// Output: Latvia

// Get country with mobile numbers only
includeLandLine = false
country := phonenumber.GetISO3166ByNumber("37165552336", includeLandLine)
fmt.Println(country.CountryName)
// Output:
```

## License
MIT
