<template>
    <div class="upploadFile">
        <input id="postFile" type="file" @change="handleFileChange" :disabled="isUploading">
        <button class="sendFile" @click="uploadFile" :disabled="!selectedFile || isUploading">
            {{ isUploading ? 'Загрузка...' : 'Отправить файл' }}
        </button>
    </div>
    <div class="uploadMessage">{{ message }}</div>
</template>

<script>
import { nextTick, onMounted, ref } from 'vue';
import { Logs } from '../store/log';

export default {
    setup() {
        const logsStore = Logs();
        const selectedFile = ref(null);
        const isUploading = ref(false);
        const uploadMessage = ref('');
        const messageClass = ref('');
        const message = ref('');

        logsStore.filter_logs = {
            level: '',
            file_name: '',
            timeStamp: '',
        };

        const handleFileChange = (event) => {
            const file = event.target.files[0];
            if (file) {
                const name = file.name.split('.');
                const extension = name[name.length - 1];
                if (extension !== 'json') {
                    message.value = 'Неверное расширение файла. Выберете лог с расширением JSON';
                    return;
                } else {
                    selectedFile.value = file;
                    message.value = '';
                }
            } else {
                selectedFile.value = null;
                message.value = '';
            }
        };

        const uploadFile = async () => {
            if (!selectedFile.value) return;

            isUploading.value = true;

            try {
                const result = await logsStore.uploadFile(selectedFile.value);
                console.log('Результат загрузки:', result);
                selectedFile.value = null;
                message.value = 'Успешно';
                event.target.value = '';
            } catch (error) {
                console.error('Ошибка:', error);
                message.value = 'Ошибка загрузки';
            } finally {
                isUploading.value = false;
            }
        };

        onMounted(async () => {

        })

        return {
            selectedFile,
            message,
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
    flex-direction: column;
    gap: 20px;
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
    transition: background-color 0.4s;
}

.sendFile:disabled {
    background-color: rgba(0, 146, 0, 0.3);
    cursor: not-allowed;
}
.uploadMessage {
    text-align: center;
}


.success {
    color: green;
}

.error {
    color: red;
}
</style>