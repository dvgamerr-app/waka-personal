<div align="center">

<img src="public/favicon.svg" width="72" alt="โลโก้ Waka Personal" />

# Waka Personal

**WakaTime backend และ dashboard สถิติการเขียนโค้ดของคุณเอง บน server ของคุณเอง**

ใช้ WakaTime plugin ตัวเดิมได้เลย แต่ทุก heartbeat ถูกเก็บในฐานข้อมูลที่คุณควบคุมเอง
ไม่มีค่าสมาชิก ไม่ต้องมีบัญชีบริการภายนอก ข้อมูลไม่ออกจากเครื่องคุณ

[![Stars](https://img.shields.io/github/stars/dvgamerr-app/waka-personal?style=for-the-badge&color=0ea5e9)](https://github.com/dvgamerr-app/waka-personal/stargazers)
[![Release](https://img.shields.io/github/v/release/dvgamerr-app/waka-personal?style=for-the-badge&color=0ea5e9)](https://github.com/dvgamerr-app/waka-personal/releases)
[![Image](https://img.shields.io/badge/ghcr.io-amd64%20%7C%20arm64-0ea5e9?style=for-the-badge&logo=docker&logoColor=white)](https://github.com/dvgamerr-app/waka-personal/pkgs/container/waka-personal)

[เว็บไซต์](https://dvgamerr.app/waka-personal/) · [เริ่มใช้งาน](#ติดตั้งใน-5-นาที) · [คู่มือ self-hosting](docs/self-hosting.md) · [English](README.md)

<img src="docs/screenshots/dashboard.png" width="230" alt="Dashboard แสดงเวลาเขียนโค้ดรายวัน ภาษา และโปรเจกต์" />
<img src="docs/screenshots/project.png" width="230" alt="รายละเอียดโปรเจกต์ ไฟล์ branch และภาษา" />
<img src="docs/screenshots/wrapped.png" width="230" alt="Wrapped สรุปประจำปีพร้อม activity matrix" />

**⭐ ถ้าช่วยให้ไม่ต้องจ่ายค่า WakaTime ช่วยกด star ให้หน่อย จะได้มีนักพัฒนาคนอื่นเจอโปรเจกต์นี้**

</div>

## ทำไมต้องใช้

- **ประวัติการเขียนโค้ดเป็นของคุณ** ทุก heartbeat อยู่ใน PostgreSQL ของคุณเอง
- **ไม่ต้องแก้ฝั่ง client** WakaTime plugin ตัวไหนก็ใช้ได้ แค่เปลี่ยน `api_url`
- **ติดตั้งด้วยคำสั่งเดียว** ตั้งค่า 2 ตัวใน `.env` แล้ว `docker compose up -d`
- **ใช้คู่กับ WakaTime ได้** `wakatime-cli` ส่งไปทั้ง Waka Personal และ wakatime.com พร้อมกันได้

## ความสามารถ

|     | ความสามารถ                      | ได้อะไร                                                                                             |
| :-: | :------------------------------ | :-------------------------------------------------------------------------------------------------- |
| 🔌  | **API แบบ WakaTime**            | Heartbeat, bulk ingest, durations, summaries, stats, status bar และ file experts                    |
| 📊  | **Dashboard ในตัว**             | เวลารายวัน ภาษา โปรเจกต์ editor และมุมมอง live ที่รีเฟรชทุก 60 วินาที                               |
| 📁  | **รายละเอียดโปรเจกต์**          | เจาะดูทีละโปรเจกต์: ไฟล์ branch ภาษา และเวลารายวัน                                                  |
| 💡  | **Insights**                    | Focus score ความยาว session การสลับบริบท และช่วงเวลาที่โฟกัสได้ดีที่สุด                             |
| 🎁  | **Wrapped**                     | สรุปประจำปีพร้อม activity matrix วันที่ทำงานหนักสุด streak และสถิติ AI coding                       |
| 🤖  | **ข้อมูล AI coding**            | AI session และจำนวน token พร้อมตารางราคา model สำหรับประเมินค่าใช้จ่าย                              |
| 📦  | **นำเข้า backup**               | นำเข้าไฟล์ export ของ WakaTime `.json` และ `.json.gz` จาก source นำเข้าซ้ำได้ไม่ซ้ำข้อมูล (SHA-256) |
| 🔐  | **Auth ผู้ใช้คนเดียว**          | API key เดียว ตรวจแบบเดียวกับที่ WakaTime plugin ส่งมา                                              |
| 🛡️  | **ค่าเริ่มต้นพร้อม production** | Rate limit, security header, health probe และ container ที่ไม่รันเป็น root                          |
| 🐳  | **Image หลายสถาปัตยกรรม**       | `linux/amd64` และ `linux/arm64` บน GitHub Container Registry                                        |

<details>
<summary><b>ภาพหน้าจอเพิ่มเติม</b></summary>

<br />

![Dashboard](docs/screenshots/dashboard.png)
![Insights](docs/screenshots/insights.png)
![Project detail](docs/screenshots/project.png)
![Wrapped](docs/screenshots/wrapped.png)

ภาพหน้าจอใช้ข้อมูลตัวอย่างที่สร้างขึ้น

</details>

## ติดตั้งใน 5 นาที

ต้องมี Docker พร้อม Compose plugin บน Linux, macOS หรือ Windows

1. ดาวน์โหลดไฟล์ compose และ template ของ environment:

   ```shell
   mkdir waka-personal && cd waka-personal
   curl -fsSLO https://raw.githubusercontent.com/dvgamerr-app/waka-personal/main/docker-compose.yml
   curl -fsSL https://raw.githubusercontent.com/dvgamerr-app/waka-personal/main/.env.example -o .env
   ```

2. แก้ `.env` เปลี่ยนค่า `REPLACE_ME` สองตัว:

   ```env
   APP_API_KEY=REPLACE_ME        # openssl rand -hex 24
   POSTGRES_PASSWORD=REPLACE_ME
   ```

3. เริ่มระบบ:

   ```shell
   docker compose up -d
   ```

4. เปิด `http://YOUR_HOST:8080` เพื่อดู dashboard แล้วตั้งค่า editor:

   ```ini
   # ~/.wakatime.cfg
   [settings]
   api_url = http://YOUR_HOST:8080/api/v1
   api_key = REPLACE_ME
   ```

Schema ของฐานข้อมูลถูกสร้างตอนเริ่มครั้งแรก เรื่อง HTTPS, backup, การอัปเกรด และการนำเข้าข้อมูลเก่า
ดูใน[คู่มือ self-hosting](docs/self-hosting.md)

## ตัวอย่างการใช้งาน

```shell
curl -u "REPLACE_ME:" http://YOUR_HOST:8080/api/v1/users/current/statusbar/today
```

WakaTime plugin ส่ง API key เป็น username ของ Basic auth จึงไม่มี password

## สถาปัตยกรรม

```text
 Editor plugins ──► wakatime-cli ──► /api/v1  ┐
                                              ├─► Go API (Fiber) ──► PostgreSQL
 Browser ─────────► dashboard ────► /api/v2  ┘        │
                                                      └─ เสิร์ฟ Astro dashboard
```

Go binary ตัวเดียว (Fiber, pgx, goose, zerolog) เสิร์ฟทั้ง API และ dashboard แบบ static (Astro + React)
จาก origin เดียวกัน เก็บข้อมูลใน PostgreSQL

## เอกสาร

- [คู่มือ self-hosting](docs/self-hosting.md) (ภาษาอังกฤษ)
- [คู่มือพัฒนา](docs/development.md) (ภาษาอังกฤษ)
- [วิธีคำนวณ activity metrics](docs/activity-metrics.md) (ภาษาอังกฤษ)

## สนับสนุนโปรเจกต์

- ⭐ [กด star](https://github.com/dvgamerr-app/waka-personal/stargazers) ให้คนอื่นเจอโปรเจกต์นี้
- 🐛 [แจ้งปัญหาหรือขอฟีเจอร์](https://github.com/dvgamerr-app/waka-personal/issues)
- 💖 [สนับสนุนผู้พัฒนา](https://github.com/sponsors/dvgamerr)

Waka Personal เป็นโปรเจกต์อิสระ ไม่มีส่วนเกี่ยวข้องกับ WakaTime
