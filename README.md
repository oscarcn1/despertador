# Despertador

Reloj despertador para Raspberry Pi con interfaz web, reproduccion de musica via Bluetooth y multiples alarmas configurables.

## Caracteristicas

- Multiples alarmas con nombre, hora (AM/PM), dias de la semana y volumen independiente
- Tres modos de reproduccion: aleatorio, secuencial o archivo unico
- Interfaz web responsive con tema oscuro
- API REST completa (CRUD de alarmas)
- Reproduccion de MP3 via `cvlc` con salida de audio por Bluetooth (PipeWire/PulseAudio)
- Persistencia de configuracion en JSON
- Servicio systemd con inicio automatico

## Requisitos

- **Hardware:** Raspberry Pi (probado en RPi 4)
- **OS:** Raspberry Pi OS / Debian
- **Go:** 1.26+
- **VLC:** 3.0+ (paquete `vlc`)
- **Audio:** PipeWire o PulseAudio configurado

Instalar VLC si no esta disponible:

```bash
sudo apt install vlc
```

## Estructura del proyecto

```
despertador/
├── main.go                      # Punto de entrada (flags: --port, --config)
├── go.mod
├── internal/
│   ├── alarm/
│   │   ├── config.go            # Modelo de datos, persistencia JSON, CRUD thread-safe
│   │   └── scheduler.go         # Scheduler (verifica cada 10s), seleccion de musica
│   ├── player/
│   │   └── player.go            # Wrapper de cvlc con loop y control de volumen
│   └── web/
│       └── handlers.go          # API REST y servidor de templates
├── web/
│   ├── templates/
│   │   └── index.html           # Interfaz web (dark theme, responsive)
│   └── static/
│       └── style.css            # Estilos
├── music/                       # Directorio default para archivos MP3
├── config.json                  # Configuracion persistente (autogenerado)
├── despertador.service          # Archivo de servicio systemd
└── .gitignore
```

## Compilacion

```bash
export PATH=$PATH:/usr/local/go/bin
go build -o despertador .
```

## Ejecucion manual

```bash
./despertador --port 8080 --config config.json
```

| Flag       | Default                                              | Descripcion               |
|------------|------------------------------------------------------|---------------------------|
| `--port`   | `8080`                                               | Puerto del servidor HTTP  |
| `--config` | `/home/oscar/Projects/despertador/config.json`       | Ruta del archivo de config|

La interfaz web estara disponible en `http://<IP_DE_LA_PI>:8080`.

## Despliegue con systemd

### 1. Copiar el archivo de servicio

```bash
sudo cp despertador.service /etc/systemd/system/
```

### 2. Editar el servicio para agregar variables de entorno de audio

El servicio necesita acceso al servidor de audio PipeWire/PulseAudio del usuario. Editar el archivo:

```bash
sudo systemctl edit despertador
```

Agregar las siguientes variables de entorno (ajustar el UID si es diferente a 1000):

```ini
[Service]
Environment=XDG_RUNTIME_DIR=/run/user/1000
Environment=DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus
Environment=PULSE_SERVER=unix:/run/user/1000/pulse/native
```

### 3. Habilitar e iniciar el servicio

```bash
sudo systemctl daemon-reload
sudo systemctl enable despertador
sudo systemctl start despertador
```

### 4. Verificar estado

```bash
sudo systemctl status despertador
journalctl -u despertador -f
```

### Rebuild y deploy rapido

```bash
export PATH=$PATH:/usr/local/go/bin
go build -o despertador . && sudo systemctl restart despertador
```

## Audio Bluetooth

Para que la alarma suene por una bocina Bluetooth:

1. Parear el dispositivo:
   ```bash
   bluetoothctl
   > scan on
   > pair <MAC>
   > trust <MAC>
   > connect <MAC>
   ```

2. Verificar que PipeWire esta activo y el sink Bluetooth esta configurado:
   ```bash
   pactl info
   pactl list sinks short
   ```

El player usa `cvlc --aout=pulse` para respetar el sink configurado en PipeWire.

## API REST

| Metodo   | Endpoint              | Descripcion                        |
|----------|-----------------------|------------------------------------|
| `GET`    | `/api/status`         | Estado completo (alarmas + ringing)|
| `POST`   | `/api/alarms`         | Crear nueva alarma                 |
| `PUT`    | `/api/alarms/{id}`    | Actualizar alarma existente        |
| `DELETE` | `/api/alarms/{id}`    | Eliminar alarma                    |
| `POST`   | `/api/dismiss`        | Apagar alarma sonando              |
| `POST`   | `/api/test/{id}`      | Probar una alarma                  |
| `GET`    | `/api/music-files?dir=` | Listar archivos MP3 en directorio|

### Ejemplo: crear una alarma

```bash
curl -X POST http://localhost:8080/api/alarms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Despertar",
    "enabled": true,
    "hour": 7,
    "minute": 0,
    "period": "AM",
    "days": [1,2,3,4,5],
    "music_dir": "/home/oscar/Projects/despertador/music",
    "volume": 80,
    "play_order": "random"
  }'
```

### Modelo de datos (AlarmEntry)

| Campo          | Tipo       | Descripcion                              |
|----------------|------------|------------------------------------------|
| `id`           | string     | ID unico (autogenerado)                  |
| `name`         | string     | Nombre de la alarma                      |
| `enabled`      | bool       | Alarma activa/inactiva                   |
| `hour`         | int (1-12) | Hora en formato 12h                      |
| `minute`       | int (0-59) | Minuto                                   |
| `period`       | string     | `"AM"` o `"PM"`                          |
| `days`         | []int      | Dias de la semana (0=Dom, 1=Lun, ..., 6=Sab) |
| `music_dir`    | string     | Directorio con archivos MP3              |
| `volume`       | int (0-100)| Volumen de reproduccion                  |
| `play_order`   | string     | `"random"`, `"sequential"` o `"single"`  |
| `selected_file`| string     | Archivo especifico (solo para modo single)|
