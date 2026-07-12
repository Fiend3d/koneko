$icon = $false

foreach ($arg in $args) {
    if ($arg -eq "icon") {
        $icon = $true
    }
}

if ($icon) {
    rsrc -ico .\assets\icon.ico
}

$output = "koneko.exe"
if (Test-Path $output) {
    Remove-Item $output
}

go build -o $output
