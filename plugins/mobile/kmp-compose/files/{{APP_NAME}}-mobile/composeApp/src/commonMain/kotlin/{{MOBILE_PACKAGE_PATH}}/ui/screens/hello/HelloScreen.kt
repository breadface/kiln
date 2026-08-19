package {{MOBILE_PACKAGE}}.ui.screens.hello

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import {{MOBILE_PACKAGE}}.viewmodel.HelloUiState
import {{MOBILE_PACKAGE}}.viewmodel.HelloViewModel

// The real integration proof: this genuinely calls backend/go-api's
// GET /api/hello (see ../../data/remote/HelloApi.kt) — not a placeholder.
// Run the backend locally (see its own README) before this will succeed.
@Composable
fun HelloScreen(viewModel: HelloViewModel = viewModel { HelloViewModel() }) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        when (val s = state) {
            is HelloUiState.Loading -> CircularProgressIndicator()
            is HelloUiState.Success -> Text(s.message, style = MaterialTheme.typography.headlineSmall)
            is HelloUiState.Error -> {
                Text("Couldn't reach the backend: ${s.reason}", style = MaterialTheme.typography.bodyMedium)
                Button(onClick = { viewModel.load() }) { Text("Retry") }
            }
        }
    }
}
