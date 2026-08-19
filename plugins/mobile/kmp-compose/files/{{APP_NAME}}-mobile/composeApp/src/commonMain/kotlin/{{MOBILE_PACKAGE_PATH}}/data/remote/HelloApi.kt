package {{MOBILE_PACKAGE}}.data.remote

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.request.get
import kotlinx.serialization.Serializable

@Serializable
data class HelloResponse(val message: String)

// A single concrete class is enough for one endpoint — reach for an
// interface (e.g. `interface HelloService { suspend fun hello(): ... }`
// with per-platform implementations) once you have many endpoints or need
// swappable fakes for tests; one call doesn't need that indirection yet.
class HelloApi(private val client: HttpClient = createHttpClient()) {
    // Tries each dev host in order and returns the first that answers.
    // Calls backend/go-api's real GET /api/hello.
    suspend fun hello(): HelloResponse {
        var lastError: Throwable? = null
        for (host in DEV_API_HOSTS) {
            try {
                return client.get("$host/api/hello").body()
            } catch (e: Throwable) {
                lastError = e
            }
        }
        throw lastError ?: IllegalStateException("no reachable host among $DEV_API_HOSTS")
    }
}
