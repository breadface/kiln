package {{MOBILE_PACKAGE}}.data.remote

import io.ktor.client.HttpClient
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.defaultRequest
import io.ktor.client.plugins.logging.LogLevel
import io.ktor.client.plugins.logging.Logging
import io.ktor.http.ContentType
import io.ktor.http.contentType
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.json.Json

// Local dev API host candidates, tried in order by HelloApi until one
// answers.
//   - 10.0.2.2   — Android Emulator's alias for the host machine.
//   - localhost / 127.0.0.1 — iOS Simulator or a Mac-native build.
//   - a LAN IP   — a physical device on the same network as your dev
//     machine. Not detectable at scaffold time; run `ipconfig getifaddr en0`
//     (macOS) and hardcode it below if you're testing on a real device.
private const val DEV_LAN_IP = "192.168.1.100"

val DEV_API_HOSTS = listOf(
    "http://10.0.2.2:{{PORT}}",
    "http://localhost:{{PORT}}",
    "http://127.0.0.1:{{PORT}}",
    "http://$DEV_LAN_IP:{{PORT}}",
)

// Not wired to an automatic dev/prod switch in this minimal starter (no
// product flavors) — swap DEV_API_HOSTS for this in ApiClient before
// shipping, or build a real build-type switch once you need one.
const val PROD_API_BASE_URL = "{{PROD_API_BASE_URL}}"

private val json = Json {
    ignoreUnknownKeys = true
    isLenient = true
    coerceInputValues = true
}

// tokenProvider is unused today (backend/go-api's example endpoint needs no
// auth) but kept as a seam for when you add it — every request gets an
// Authorization header automatically once tokenProvider returns non-null.
fun createHttpClient(tokenProvider: () -> String? = { null }): HttpClient = HttpClient {
    install(ContentNegotiation) { json(json) }
    install(Logging) { level = LogLevel.NONE }
    defaultRequest {
        contentType(ContentType.Application.Json)
        tokenProvider()?.let { headers.append("Authorization", "Bearer $it") }
    }
}
