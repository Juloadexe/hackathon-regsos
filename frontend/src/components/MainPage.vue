<template>
    <div class="upploadFile">
        <input type="file" @change="handleFileChange" :disabled="isUploading">
        <button class="sendFile" @click="uploadFile" :disabled="!selectedFile || isUploading">
            {{ isUploading ? 'Загрузка...' : 'Отправить файл' }}
        </button>
    </div>
</template>

<script>
import { ref } from 'vue';
import { Logs } from '../store/log';

export default {
    setup() {
        const logsStore = Logs();
        const selectedFile = ref(null);
        const isUploading = ref(false);
        const uploadMessage = ref('');
        const messageClass = ref('');

        logsStore.filter_logs = {
            level: '',
            file_name: '',
            timeStamp: '',
        };

        const handleFileChange = (event) => {
            const file = event.target.files[0];
            if (file) {
                selectedFile.value = file;
            }
        };

        const uploadFile = async () => {
            if (!selectedFile.value) return;

            isUploading.value = true;

            try {
                const result = await logsStore.uploadFile(selectedFile.value);
                console.log('Результат загрузки:', result);

                selectedFile.value = null;
                event.target.value = '';

            } catch (error) {
                console.error('Ошибка:', error);
            } finally {
                isUploading.value = false;
            }
        };

        return {
            selectedFile,
            isUploading,
            uploadMessage,
            messageClass,
            handleFileChange,
            uploadFile
        }
    }
}
</script>

<style scoped>

.upploadFile {
    display: flex;
    gap: 10px;
    justify-content: center;
    padding: 30px;
    align-items: center;
}

.sendFile {
    padding: 10px;
    border: none;
    border-radius: 5px;
    background-color: green;
    color: white;
    font-size: 16px;
}


.success {
    color: green;
}

.error {
    color: red;
}
</style>