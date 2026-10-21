# Package mgmt.lib holds helpers shared by every policy: date arithmetic on
# YYYY-MM-DD strings, durations such as "90d" and "2w", and the finding shape.
package mgmt.lib

_day_ns := ((24 * 60) * 60) * 1000000000

_ns(date) := time.parse_ns("2006-01-02", date)

# add_days("2026-12-29", 5) == "2027-01-03"
add_days(date, n) := time.format([_ns(date) + (n * _day_ns), "UTC", "2006-01-02"])

# days("90d") == 90, days("2w") == 14
days(duration) := to_number(trim_suffix(duration, "d")) if endswith(duration, "d")

days(duration) := to_number(trim_suffix(duration, "w")) * 7 if endswith(duration, "w")

# days_between(a, b) is b - a in whole days.
days_between(a, b) := round((_ns(b) - _ns(a)) / _day_ns)

max_date(a, b) := a if a >= b

max_date(a, b) := b if a < b

min_date(a, b) := a if a <= b

min_date(a, b) := b if a > b

# finding builds a deny or warn entry; msg duplicates message for Conftest.
finding(id, person, file, line, message) := {
	"id": id,
	"person": person,
	"file": file,
	"line": line,
	"message": message,
	"msg": message,
}
