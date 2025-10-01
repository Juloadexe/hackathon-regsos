<template>
    <div class="upploadFile">
        <input id="postFile" type="file" @change="handleFileChange" :disabled="isUploading">
        <button class="sendFile" @click="uploadFile" :disabled="!selectedFile || isUploading">
            {{ isUploading ? 'Загрузка...' : 'Отправить файл' }}
        </button>
    </div>
    <div class="uploadMessage">{{ message }}</div>


    <div class="filter">
        <div class="filter__wrapper">
            <select v-model="logsStore.filter_logs.level" class="filter__input">
                <option value=""></option>
                <option value="info">info</option>
                <option value="debug">debug</option>
                <option value="trace">trace</option>
                <option value="error">error</option>
            </select>
            <label class="filter__label">Тип лога</label>
        </div>
        <div class="filter__wrapper">
            <input type="date" class="filter__input" v-model="logsStore.filter_logs.since">
            <label class="filter__label">От время</label>
        </div>
        <div class="filter__wrapper">
            <input type="date" class="filter__input" v-model="logsStore.filter_logs.until">
            <label class="filter__label">До время</label>
        </div>
        <button id="confButton" @click="applyFiters">Применить</button>
    </div>
    <table class="table">
        <thead>
            <tr>
                <th>Уровень</th>
                <th>Сообщение</th>
                <th>Module</th>
                <th>Caller</th>
                <th>Дата создания</th>
            </tr>
        </thead>
        <tbody>
            <tr v-for="value in logsStore.logsData.logs">
                <td :style="{ color: getColor(value.Level), backgroundColor: getBack(value.Level) }">
                    {{ value.Level }}
                </td>
                <td>
                    {{ value.Message }}
                </td>
                <td>
                    {{ value.Module }}
                </td>
                <td>
                    {{ value.Caller }}
                </td>
                <td>
                    {{ formatTime(value.Timestamp) }}
                </td>
            </tr>
        </tbody>
    </table>

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
            since: '',
            until: '',
        };

        const handleFileChange = (event) => {
            const file = event.target.files[0];
            if (file) {
                const name = file.name.split('.');
                const extension = name[name.length - 1];
                if (extension !== 'json') {
                    selectedFile.value = null;
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

        function applyFiters() {
            console.log(logsStore.filter_logs);

            logsStore.setFiters(logsStore.filter_logs);
        }

        function getColor(type) {
            switch(type) {
                case 'info':
                    return '#4E84A8'
                case 'debug':
                    return '#837544'
                case 'trace':
                    return '#6b7280'
                case 'error':
                    return '#A23038'
                default:
                    return '#afa4a4';
            }
        }

        function getBack(type) {
            switch (type) {
                case 'info':
                    return '#CCE8F4'
                case 'debug':
                    return '#F8F3D6'
                case 'trace':
                    return '#f9fafb'
                case 'error':
                    return '#EBC8C4'
                default:
                    return '#afa4a4';
            }  
        }

        const formatTime = (timestamp) => {
            const date = new Date(timestamp);
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                hour12: false
            }).replace(',', '');
        };

        onMounted(async () => {
           await logsStore.getLogsData();
        })

        return {
            selectedFile,
            message,
            isUploading,
            uploadMessage,
            messageClass,
            handleFileChange,
            uploadFile,
            logsStore,
            formatTime,
            applyFiters,
            getColor,
            getBack,
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

.table {
    width: 100%;
    margin-bottom: 20px;
    border: 1px solid #dddddd;
    border-collapse: collapse;
}

.filter {
    display: flex;
    gap: 10px;
    margin-bottom: 20px;
    justify-content: center;
    align-items: center;
}

.filter__wrapper {
    position: relative;
}

.filter__input {
    padding: 10px;
    padding-top: 13px;
    padding-bottom: 5px;
    border: none;
    border-radius: 5px;
    font-size: 1em;
    background-color: #ffffff;
    color: #333;
    cursor: text;
    border: 1px solid #000000;
    transition: background-color 0.3s, color 0.3s;
    width: 250px;
    height: 45px;
    box-sizing: border-box;
}

.filter__label {
    position: absolute;
    font-size: 12px;
    top: 0px;
    left: 11px;
}

#confButton {
    background-color: green;
    color: white;
    font-size: 16px;
    border: none;
    border-radius: 5px;
    padding: 10px;
}

th {
    text-align: center;
    font-size: 1em;
    width: 100px;
    border: 1px solid #dddddd;
}

td {
    text-align: center;
    font-size: 1em;
    width: 100px;
    border: 1px solid #dddddd;
}

.success {
    color: green;
}

.error {
    color: red;
}
</style>