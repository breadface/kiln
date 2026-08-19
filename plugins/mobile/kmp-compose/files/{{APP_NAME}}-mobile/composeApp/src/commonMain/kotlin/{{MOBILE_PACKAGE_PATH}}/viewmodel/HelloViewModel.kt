package {{MOBILE_PACKAGE}}.viewmodel

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import {{MOBILE_PACKAGE}}.data.remote.HelloApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed interface HelloUiState {
    data object Loading : HelloUiState
    data class Success(val message: String) : HelloUiState
    data class Error(val reason: String) : HelloUiState
}

class HelloViewModel(private val api: HelloApi = HelloApi()) : ViewModel() {
    private val _state = MutableStateFlow<HelloUiState>(HelloUiState.Loading)
    val state: StateFlow<HelloUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        _state.value = HelloUiState.Loading
        viewModelScope.launch {
            _state.value = try {
                HelloUiState.Success(api.hello().message)
            } catch (e: Throwable) {
                HelloUiState.Error(e.message ?: "unknown error")
            }
        }
    }
}
