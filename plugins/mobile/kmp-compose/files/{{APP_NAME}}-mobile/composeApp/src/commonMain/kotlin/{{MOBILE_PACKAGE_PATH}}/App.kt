package {{MOBILE_PACKAGE}}

import androidx.compose.runtime.Composable
import {{MOBILE_PACKAGE}}.ui.screens.hello.HelloScreen
import {{MOBILE_PACKAGE}}.ui.theme.AppTheme

@Composable
fun App() {
    AppTheme {
        HelloScreen()
    }
}
