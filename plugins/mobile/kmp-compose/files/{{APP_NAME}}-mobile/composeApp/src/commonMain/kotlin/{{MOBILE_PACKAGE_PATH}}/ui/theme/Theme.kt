package {{MOBILE_PACKAGE}}.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable

// Material 3's own default color scheme and typography — deliberately not
// customized here. Replace with your own brand palette when you have one;
// see https://m3.material.io/styles/color/dynamic-color/overview for how
// lightColorScheme()/darkColorScheme() are usually built from a seed color.
private val Colors = lightColorScheme()

@Composable
fun AppTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = Colors,
        typography = Typography(),
        content = content,
    )
}
