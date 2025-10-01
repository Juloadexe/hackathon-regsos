import { defineStore } from "pinia";
import axios from "axios";

export const Logs = defineStore('logs', {
    state: () => ({
        logsData: [],
        filter_logs: [],
        url: 'http://localhost:8080/api'
    }),
    actions: {
        async uploadFile(file) {
            try {
                const formData = new FormData()
                formData.append('file', file)
                const response = await axios.post(`${this.url}/logs`, formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                })
                
                return response.data
            } catch (error) {
                console.error('Ошибка загрузки файла:', error)
                throw error
            }
        },
        async getLogsData() {
            try {
                const response = await axios(`${this.url}/dataLogger`)
                this.logsData = response.data;
            } catch (error) {
                console.error('Ошибка получения данных логов:', error)
                throw error;
            }
        }
    }
})